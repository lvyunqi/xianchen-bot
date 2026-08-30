package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

func (g *Game) bankAccount(player *model.Player) (model.BankAccount, error) {
	row := model.BankAccount{PlayerID: player.ID, CreditScore: 600}
	if err := g.store.DB.Where("player_id = ?", player.ID).FirstOrCreate(&row).Error; err != nil {
		return row, err
	}
	if row.CreditScore <= 0 {
		row.CreditScore = 600
		_ = g.store.DB.Model(&row).Update("credit_score", row.CreditScore).Error
	}
	return g.accrueBankOverdue(row)
}

func (g *Game) accrueBankOverdue(row model.BankAccount) (model.BankAccount, error) {
	if row.SilverPrincipal <= 0 || row.LoanDueAt == nil || !time.Now().After(*row.LoanDueAt) {
		return row, nil
	}
	from := *row.LoanDueAt
	if row.InterestCalculatedAt != nil && row.InterestCalculatedAt.After(from) {
		from = *row.InterestCalculatedAt
	}
	days := int64(time.Since(from) / (24 * time.Hour))
	if days <= 0 {
		return row, nil
	}
	rate := max64(g.settingInt("bank.overdue_daily_basis_points", 100), 0)
	extra := safeBasisPointCharge(row.SilverPrincipal, rate*days)
	now := time.Now()
	credit := maxInt(row.CreditScore-int(min64(days, 20)), 300)
	if err := g.store.DB.Model(&row).Updates(map[string]any{
		"silver_interest": row.SilverInterest + extra, "interest_calculated_at": &now, "credit_score": credit,
	}).Error; err != nil {
		return row, err
	}
	row.SilverInterest += extra
	row.InterestCalculatedAt = &now
	row.CreditScore = credit
	return row, nil
}

func safeBasisPointCharge(amount, basisPoints int64) int64 {
	if amount <= 0 || basisPoints <= 0 {
		return 0
	}
	value := float64(amount) * float64(basisPoints) / 10000
	if value >= float64(math.MaxInt64) {
		return math.MaxInt64
	}
	return int64(math.Ceil(value))
}

func (g *Game) bankCreditLimit(player *model.Player, account model.BankAccount) int64 {
	sequence, err := g.playerRealmSequence(player)
	if err != nil || sequence < 1 {
		sequence = 1
	}
	creditBonus := int64(maxInt(account.CreditScore-500, 0)) * 10
	return 1000 + int64(sequence)*1000 + max64(player.Reputation, 0)*10 + max64(player.Merit, 0)*5 + creditBonus
}

func (g *Game) bankOverview(player *model.Player) (GameResult, bool, error) {
	account, err := g.bankAccount(player)
	if err != nil {
		return GameResult{}, true, err
	}
	debt := account.SilverPrincipal + account.SilverInterest
	limit := g.bankCreditLimit(player, account)
	available := max64(limit-debt, 0)
	due := "无借款"
	if account.LoanDueAt != nil && debt > 0 {
		due = account.LoanDueAt.Format("2006-01-02 15:04")
		if time.Now().After(*account.LoanDueAt) {
			due += "（已逾期）"
		}
	}
	content := fmt.Sprintf("道号：%s · 钱庄信誉%d/850\n━━━━━━━━━━━\n【随身财物】\n银币：%d · 灵石：%d · 仙金：%d\n【洞天存契】\n银币存款：%d · 灵石存款：%d\n【仙盟借契】\n银币本金：%d · 待还利息：%d · 合计：%d\n信誉额度：%d · 尚可借：%d\n还款期限：%s\n━━━━━━━━━━━\n钱庄只贷银币；仙金属于充值货币，不可存入、借出或通过利息生成。存款用于与随身余额隔离保管，当前不产生收益。", player.DaoName, account.CreditScore, player.SilverCoins, player.SpiritStones, player.ImmortalJade, account.SilverDeposit, account.SpiritStoneDeposit, account.SilverPrincipal, account.SilverInterest, debt, limit, available, due)
	return GameResult{Title: "🏦 仙盟钱庄", Content: content, Actions: []string{"存款 银币 100", "取款 银币 100", "借款 100", "还款 100", "钱庄账簿", "钱庄规则", "银币商城", "货币"}}, true, nil
}

func parseBankCurrencyAmount(raw string) (string, int64, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) == 0 {
		return "", 0, fmt.Errorf("请输入正整数数量")
	}
	currency := "银币"
	amountText := parts[0]
	if len(parts) >= 2 {
		currency = parts[0]
		amountText = parts[1]
	}
	if currency != "银币" && currency != "灵石" {
		return "", 0, fmt.Errorf("钱庄只支持银币或灵石存取；仙金不可进入钱庄")
	}
	amount, err := strconv.ParseInt(amountText, 10, 64)
	if err != nil || amount <= 0 {
		return "", 0, fmt.Errorf("数量必须是正整数")
	}
	return currency, amount, nil
}

func (g *Game) bankDeposit(player *model.Player, raw string) (GameResult, bool, error) {
	currency, amount, err := parseBankCurrencyAmount(raw)
	if err != nil {
		return GameResult{Title: "🏦 存契格式", Content: err.Error(), Actions: []string{"存款 银币 100", "存款 灵石 100", "钱庄"}}, true, nil
	}
	account, err := g.bankAccount(player)
	if err != nil {
		return GameResult{}, true, err
	}
	walletColumn, depositColumn := "silver_coins", "silver_deposit"
	wallet := player.SilverCoins
	deposit := account.SilverDeposit
	if currency == "灵石" {
		walletColumn, depositColumn = "spirit_stones", "spirit_stone_deposit"
		wallet, deposit = player.SpiritStones, account.SpiritStoneDeposit
	}
	if wallet < amount {
		return GameResult{Title: "🏦 随身余额不足", Content: fmt.Sprintf("存入%s×%d需要随身持有足额%s，当前只有%d。", currency, amount, currency, wallet), Actions: []string{"货币", "钱庄"}}, true, nil
	}
	newDeposit := deposit + amount
	if newDeposit < deposit {
		return GameResult{Title: "🏦 数额过大", Content: "本次存款超过安全记账范围，请拆分操作。"}, true, nil
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Player{}).Where("id = ? AND "+walletColumn+" >= ?", player.ID, amount).Update(walletColumn, gorm.Expr(walletColumn+" - ?", amount))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errInsufficientCurrency
		}
		if err := tx.Model(&account).Update(depositColumn, gorm.Expr(depositColumn+" + ?", amount)).Error; err != nil {
			return err
		}
		return tx.Create(&model.BankTransaction{PlayerID: player.ID, Type: "存款", Currency: currency, Amount: amount, BalanceAfter: newDeposit, DebtAfter: account.SilverPrincipal + account.SilverInterest, Description: "存入仙盟钱庄"}).Error
	})
	if err == errInsufficientCurrency {
		return GameResult{Title: "🏦 随身余额不足", Content: "余额刚刚发生变化，请重新查看钱庄。", Actions: []string{"钱庄"}}, true, nil
	}
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🏦 存契完成", Content: fmt.Sprintf("存入：%s×%d\n存款余额：%d\n随身余额：%d", currency, amount, newDeposit, wallet-amount), Actions: []string{"钱庄", "取款 " + currency + " " + strconv.FormatInt(amount, 10), "钱庄账簿"}}, true, nil
}

func (g *Game) bankWithdraw(player *model.Player, raw string) (GameResult, bool, error) {
	currency, amount, err := parseBankCurrencyAmount(raw)
	if err != nil {
		return GameResult{Title: "🏦 取契格式", Content: err.Error(), Actions: []string{"取款 银币 100", "取款 灵石 100", "钱庄"}}, true, nil
	}
	account, err := g.bankAccount(player)
	if err != nil {
		return GameResult{}, true, err
	}
	walletColumn, depositColumn := "silver_coins", "silver_deposit"
	deposit := account.SilverDeposit
	if currency == "灵石" {
		walletColumn, depositColumn, deposit = "spirit_stones", "spirit_stone_deposit", account.SpiritStoneDeposit
	}
	if deposit < amount {
		return GameResult{Title: "🏦 存款不足", Content: fmt.Sprintf("%s存款只有%d，无法取出%d。", currency, deposit, amount), Actions: []string{"钱庄", "存款 " + currency + " 100"}}, true, nil
	}
	newDeposit := deposit - amount
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.BankAccount{}).Where("id = ? AND "+depositColumn+" >= ?", account.ID, amount).Update(depositColumn, gorm.Expr(depositColumn+" - ?", amount))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errInsufficientCurrency
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Update(walletColumn, gorm.Expr(walletColumn+" + ?", amount)).Error; err != nil {
			return err
		}
		return tx.Create(&model.BankTransaction{PlayerID: player.ID, Type: "取款", Currency: currency, Amount: amount, BalanceAfter: newDeposit, DebtAfter: account.SilverPrincipal + account.SilverInterest, Description: "取回随身钱袋"}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🏦 取契完成", Content: fmt.Sprintf("取出：%s×%d\n存款余额：%d", currency, amount, newDeposit), Actions: []string{"钱庄", "存款 " + currency + " " + strconv.FormatInt(amount, 10), "钱庄账簿"}}, true, nil
}

func parsePositiveAmount(raw string) (int64, error) {
	amount, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || amount <= 0 {
		return 0, fmt.Errorf("数量必须是正整数")
	}
	return amount, nil
}

func (g *Game) bankBorrow(player *model.Player, raw string) (GameResult, bool, error) {
	amount, err := parsePositiveAmount(raw)
	if err != nil {
		return GameResult{Title: "🏦 借契格式", Content: "请输入：`借款 数量`。钱庄只提供银币借款。", Actions: []string{"钱庄规则", "钱庄"}}, true, nil
	}
	account, err := g.bankAccount(player)
	if err != nil {
		return GameResult{}, true, err
	}
	if account.LoanDueAt != nil && time.Now().After(*account.LoanDueAt) && account.SilverPrincipal+account.SilverInterest > 0 {
		return GameResult{Title: "🏦 逾期借契未清", Content: "旧借契已经逾期，须先清偿本金与利息，才可再次借款。", Actions: []string{"还款 100", "钱庄", "钱庄规则"}}, true, nil
	}
	debt := account.SilverPrincipal + account.SilverInterest
	limit := g.bankCreditLimit(player, account)
	if amount > max64(limit-debt, 0) {
		return GameResult{Title: "🏦 超出信誉额度", Content: fmt.Sprintf("当前额度%d，已有待还%d，本次最多可借%d银币。提高境界、声望、功德并按期还款可逐步提高额度。", limit, debt, max64(limit-debt, 0)), Actions: []string{"钱庄", "任务菜单", "修炼"}}, true, nil
	}
	interest := safeBasisPointCharge(amount, max64(g.settingInt("bank.loan_interest_basis_points", 500), 0))
	dueAt := time.Now().Add(time.Duration(max64(g.settingInt("bank.loan_days", 7), 1)) * 24 * time.Hour)
	if account.LoanDueAt != nil && debt > 0 {
		dueAt = *account.LoanDueAt
	}
	now := time.Now()
	newDebt := debt + amount + interest
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&account).Updates(map[string]any{
			"silver_principal": account.SilverPrincipal + amount, "silver_interest": account.SilverInterest + interest,
			"loan_due_at": &dueAt, "interest_calculated_at": &now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Player{}).Where("id = ?", player.ID).Update("silver_coins", gorm.Expr("silver_coins + ?", amount)).Error; err != nil {
			return err
		}
		return tx.Create(&model.BankTransaction{PlayerID: player.ID, Type: "借款", Currency: "银币", Amount: amount, BalanceAfter: player.SilverCoins + amount, DebtAfter: newDebt, Description: fmt.Sprintf("基础利息%d，限期%s", interest, dueAt.Format("2006-01-02"))}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	return GameResult{Title: "🏦 仙盟借契立成", Content: fmt.Sprintf("借得银币：%d\n本笔利息：%d\n累计待还：%d\n还款期限：%s\n━━━━━━━━━━━\n银币已进入随身钱袋；逾期后按完整逾期日追加利息并降低信誉。", amount, interest, newDebt, dueAt.Format("2006-01-02 15:04")), Actions: []string{"还款 " + strconv.FormatInt(newDebt, 10), "钱庄", "钱庄账簿", "银币商城"}}, true, nil
}

func (g *Game) bankRepay(player *model.Player, raw string) (GameResult, bool, error) {
	amount, err := parsePositiveAmount(raw)
	if err != nil {
		return GameResult{Title: "🏦 还契格式", Content: "请输入：`还款 数量`。还款使用随身银币。", Actions: []string{"钱庄"}}, true, nil
	}
	account, err := g.bankAccount(player)
	if err != nil {
		return GameResult{}, true, err
	}
	debt := account.SilverPrincipal + account.SilverInterest
	if debt <= 0 {
		return GameResult{Title: "🏦 无待还借契", Content: "当前没有银币借款，无需还款。", Actions: []string{"钱庄", "借款 100"}}, true, nil
	}
	if amount > debt {
		amount = debt
	}
	if player.SilverCoins < amount {
		return GameResult{Title: "🏦 银币不足", Content: fmt.Sprintf("本次需用银币%d，随身只有%d。可先取出银币存款或完成签到、任务与竞技俸禄。", amount, player.SilverCoins), Actions: []string{"取款 银币 " + strconv.FormatInt(amount, 10), "签到", "任务菜单", "竞技奖励"}}, true, nil
	}
	interestPaid := min64(amount, account.SilverInterest)
	principalPaid := amount - interestPaid
	newInterest := account.SilverInterest - interestPaid
	newPrincipal := account.SilverPrincipal - principalPaid
	newDebt := newPrincipal + newInterest
	updates := map[string]any{"silver_principal": newPrincipal, "silver_interest": newInterest}
	if newDebt == 0 {
		updates["loan_due_at"] = nil
		updates["interest_calculated_at"] = nil
		updates["credit_score"] = minInt(account.CreditScore+5, 850)
	}
	err = g.store.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Player{}).Where("id = ? AND silver_coins >= ?", player.ID, amount).Update("silver_coins", gorm.Expr("silver_coins - ?", amount))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errInsufficientCurrency
		}
		if err := tx.Model(&account).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Create(&model.BankTransaction{PlayerID: player.ID, Type: "还款", Currency: "银币", Amount: amount, BalanceAfter: player.SilverCoins - amount, DebtAfter: newDebt, Description: fmt.Sprintf("偿付利息%d、本金%d", interestPaid, principalPaid)}).Error
	})
	if err != nil {
		return GameResult{}, true, err
	}
	status := fmt.Sprintf("剩余本金：%d · 剩余利息：%d", newPrincipal, newInterest)
	if newDebt == 0 {
		status = "借契已经全部清偿，钱庄信誉+5。"
	}
	return GameResult{Title: "🏦 还契完成", Content: fmt.Sprintf("本次归还：%d银币\n其中利息：%d · 本金：%d\n%s", amount, interestPaid, principalPaid, status), Actions: []string{"钱庄", "钱庄账簿", "借款 100"}}, true, nil
}

func (g *Game) bankLedger(player *model.Player, raw string) (GameResult, bool, error) {
	page := maxInt(int(parsePositiveInt(strings.TrimSpace(raw), 1)), 1)
	const pageSize = 8
	var total int64
	if err := g.store.DB.Model(&model.BankTransaction{}).Where("player_id = ?", player.ID).Count(&total).Error; err != nil {
		return GameResult{}, true, err
	}
	pages := maxInt((int(total)+pageSize-1)/pageSize, 1)
	if page > pages {
		page = pages
	}
	var rows []model.BankTransaction
	if err := g.store.DB.Where("player_id = ?", player.ID).Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return GameResult{}, true, err
	}
	lines := []string{fmt.Sprintf("第%d/%d页 · 共%d笔", page, pages, total), "━━━━━━━━━━━"}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%s · %s%s×%d\n%s\n存契/随身余额：%d · 待还：%d", row.CreatedAt.Format("01-02 15:04"), row.Type, row.Currency, row.Amount, row.Description, row.BalanceAfter, row.DebtAfter), "━━━━━━━")
	}
	if len(rows) == 0 {
		lines = append(lines, "尚无钱庄往来记录。")
	}
	actions := []string{"钱庄"}
	if page > 1 {
		actions = append(actions, fmt.Sprintf("钱庄账簿 %d", page-1))
	}
	if page < pages {
		actions = append(actions, fmt.Sprintf("钱庄账簿 %d", page+1))
	}
	return GameResult{Title: "🏦 钱庄账簿", Content: strings.Join(lines, "\n"), Actions: actions}, true, nil
}

func (g *Game) bankRules(player *model.Player) GameResult {
	account, _ := g.bankAccount(player)
	return GameResult{Title: "🏦 仙盟借契律", Content: fmt.Sprintf("【存取】银币、灵石可随时存取，不限次数，不收手续费；存款只作隔离保管，不产生收益。\n【禁区】仙金为充值货币，不能存款、借款或孳息。\n【额度】当前信誉%d，额度%d银币；由境界、声望、功德和履约信誉共同决定。\n【利息】借款时一次计入基础利息%.2f%%，期限%d天。\n【逾期】每完整逾期日按本金追加%.2f%%利息并降低信誉；逾期借契清偿前不能再借。\n【还款】优先偿还利息，再归还本金；足额清偿后信誉+5，最高850。", account.CreditScore, g.bankCreditLimit(player, account), float64(g.settingInt("bank.loan_interest_basis_points", 500))/100, g.settingInt("bank.loan_days", 7), float64(g.settingInt("bank.overdue_daily_basis_points", 100))/100), Actions: []string{"钱庄", "借款 100", "还款 100", "钱庄账簿"}}
}

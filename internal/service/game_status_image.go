package service

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"xianlv/internal/model"
)

//go:embed assets/status_template.jpg
var statusTemplateImage []byte

const (
	statusPortraitCenterX = 425
	statusPortraitCenterY = 634
	statusPortraitRadius  = 184
	statusIdentityCenterX = 425
)

type statusRelations struct {
	Couple string
	Sect   string
	Master string
	Pet    string
}

func (g *Game) renderStatusImage(player *model.Player, stamina, staminaMaximum int64) (string, error) {
	templateImage, _, err := image.Decode(bytes.NewReader(statusTemplateImage))
	if err != nil {
		return "", fmt.Errorf("解析状态底图: %w", err)
	}
	canvas := image.NewRGBA(templateImage.Bounds())
	drawImage(canvas, templateImage)
	medicineBonus := g.activeItemBonuses(player.ID)
	physicalDefense, magicDefense, agility, daoHeart := medicineAdjustedDisplayStats(player, medicineBonus)
	medicineState := ""
	if activeMedicineBonusText(medicineBonus) != "" {
		medicineState = " · 药力生效"
	}

	statusFont, err := loadStatusFont(g.settingText("image.status_font_name", ""))
	if err != nil {
		return "", err
	}

	if avatar, avatarErr := loadStatusAvatar(g.playerAvatarURL(player)); avatarErr == nil && avatar != nil {
		drawCircularAvatar(canvas, avatar, image.Pt(statusPortraitCenterX, statusPortraitCenterY), statusPortraitRadius)
	}

	panel := color.RGBA{R: 39, G: 58, B: 51, A: 255}
	fillOverlay(canvas, image.Rect(858, 365, 2348, 505), panel)
	fillOverlay(canvas, image.Rect(858, 585, 2348, 735), panel)
	fillOverlay(canvas, image.Rect(858, 838, 1307, 1062), panel)
	fillOverlay(canvas, image.Rect(1334, 838, 1807, 1062), panel)
	fillOverlay(canvas, image.Rect(1830, 838, 2352, 1062), panel)
	fillOverlay(canvas, image.Rect(323, 944, 548, 1002), color.RGBA{R: 238, G: 223, B: 161, A: 255})

	gold := color.RGBA{R: 248, G: 235, B: 184, A: 255}
	ivory := color.RGBA{R: 245, G: 246, B: 226, A: 255}
	muted := color.RGBA{R: 204, G: 217, B: 198, A: 255}
	dark := color.RGBA{R: 40, G: 64, B: 55, A: 255}

	drawCenteredFittedText(canvas, statusFont, player.DaoName, statusIdentityCenterX, 986, 196, 38, 21, dark)
	if strings.TrimSpace(player.Title) != "" {
		drawCenteredFittedText(canvas, statusFont, player.Title, statusIdentityCenterX, 1072, 390, 31, 20, gold)
	}

	cultivationRatio := fraction(player.Cultivation, max64(player.CultivationRequired, 1))
	healthRatio := fraction(player.Health, max64(player.MaxHealth, 1))
	manaRatio := fraction(player.Mana, max64(player.MaxMana, 1))

	drawFittedText(canvas, statusFont, fmt.Sprintf("境界  %s · %d层", displayOr(player.RealmName, "未入境"), maxInt(player.RealmLevel, 1)), 920, 410, 550, 36, 25, ivory)
	drawFittedText(canvas, statusFont, "灵根  "+displayOr(player.SpiritualRoot, "尚未觉醒")+fmt.Sprintf(" · 纯度%d", player.RootQuality), 920, 474, 550, 32, 22, muted)
	drawFittedText(canvas, statusFont, fmt.Sprintf("修为  %s / %s  %.1f%%", statusNumber(player.Cultivation), statusNumber(player.CultivationRequired), cultivationRatio*100), 1515, 410, 760, 34, 23, ivory)
	drawProgressBar(canvas, image.Rect(1518, 430, 2255, 444), cultivationRatio, color.RGBA{R: 214, G: 171, B: 71, A: 255})
	drawFittedText(canvas, statusFont, fmt.Sprintf("等级 LV%d · 经验 %s/%s · 战力 %s · 状态 %s%s", maxInt(player.Level, 1), statusNumber(max64(player.Experience, 0)), statusNumber(model.PlayerExperienceRequired(maxInt(player.Level, 1))), statusNumber(player.CombatPower), statusState(player), medicineState), 1515, 482, 790, 31, 18, gold)

	drawFittedText(canvas, statusFont, fmt.Sprintf("气血  %s / %s", statusNumber(player.Health), statusNumber(player.MaxHealth)), 910, 630, 430, 30, 21, ivory)
	drawProgressBar(canvas, image.Rect(913, 646, 1325, 658), healthRatio, color.RGBA{R: 190, G: 66, B: 60, A: 255})
	drawFittedText(canvas, statusFont, fmt.Sprintf("法力  %s / %s", statusNumber(player.Mana), statusNumber(player.MaxMana)), 910, 698, 430, 30, 21, ivory)
	drawProgressBar(canvas, image.Rect(913, 714, 1325, 726), manaRatio, color.RGBA{R: 71, G: 145, B: 193, A: 255})
	drawFittedText(canvas, statusFont, fmt.Sprintf("物攻 %s · 法强 %s", statusNumber(player.PhysicalAttack), statusNumber(player.MagicAttack)), 1400, 630, 450, 29, 20, ivory)
	drawFittedText(canvas, statusFont, fmt.Sprintf("物抗 %s · 法抗 %s", statusNumber(physicalDefense), statusNumber(magicDefense)), 1400, 700, 450, 29, 20, ivory)
	drawFittedText(canvas, statusFont, fmt.Sprintf("身法 %s · 闪避 %.1f%%", statusNumber(agility), player.DodgeRate*100), 1890, 630, 410, 29, 20, ivory)
	drawFittedText(canvas, statusFont, fmt.Sprintf("道心 %d · 神识 %d · 运气 %d/%d", daoHeart, player.Spirit, normalizedPlayerLuck(player.Luck), maximumPlayerLuck), 1890, 700, 410, 29, 18, ivory)

	relations := g.statusRelations(player)
	drawStatusRows(canvas, statusFont, 895, []string{
		"仙侣  " + relations.Couple,
		"宗门  " + relations.Sect,
		"师承  " + relations.Master,
		"灵兽  " + relations.Pet,
	}, 900, 350, 29, ivory)
	drawStatusRows(canvas, statusFont, 895, []string{
		"灵石  " + statusNumber(player.SpiritStones),
		"银币  " + statusNumber(player.SilverCoins),
		"仙金  " + statusNumber(player.ImmortalJade),
		"竞技币  " + statusNumber(player.ArenaCoins),
	}, 1370, 390, 29, ivory)
	drawStatusRows(canvas, statusFont, 895, []string{
		"道号  " + player.DaoName + " · " + displayPlayerGender(player.Gender),
		fmt.Sprintf("账号  %s · 体力 %s/%s", compactAccountID(player.AccountID), statusNumber(stamina), statusNumber(staminaMaximum)),
		fmt.Sprintf("道龄  %d年 · 仙缘 %s", player.Age, statusNumber(player.ImmortalAffinity)),
		"位置  " + displayOr(player.Location, "未知之地"),
	}, 1862, 435, 28, ivory)

	directory := filepath.Join(g.store.RuntimeCacheDirectory(), "status")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("创建状态图缓存目录: %w", err)
	}
	file, err := os.CreateTemp(directory, "xianchen-status-*.jpg")
	if err != nil {
		return "", fmt.Errorf("创建状态图: %w", err)
	}
	path := file.Name()
	if err := jpeg.Encode(file, canvas, &jpeg.Options{Quality: 91}); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("编码状态图: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func (g *Game) statusRelations(player *model.Player) statusRelations {
	relations := statusRelations{Couple: "未结仙缘", Sect: displayOr(player.SectName, "散修"), Master: "暂无师承", Pet: "未出战"}
	var couple model.Couple
	if g.store.DB.Where("(player_a_id = ? OR player_b_id = ?) AND status <> ?", player.ID, player.ID, "已解除").Order("id DESC").First(&couple).Error == nil {
		if couple.PlayerAID == player.ID {
			relations.Couple = displayOr(couple.PlayerBName, "道侣")
		} else {
			relations.Couple = displayOr(couple.PlayerAName, "道侣")
		}
	}
	var mentorship model.Mentorship
	if g.store.DB.Where("(master_id = ? OR disciple_id = ?) AND status = ?", player.ID, player.ID, "正常").Order("id DESC").First(&mentorship).Error == nil {
		otherID := mentorship.MasterID
		prefix := "师尊·"
		if mentorship.MasterID == player.ID {
			otherID = mentorship.DiscipleID
			prefix = "弟子·"
		}
		var other model.Player
		if g.store.DB.Select("dao_name").First(&other, otherID).Error == nil {
			relations.Master = prefix + other.DaoName
		}
	}
	var pet model.Pet
	query := g.store.DB.Where("player_id = ? AND active = ?", player.ID, true)
	if player.ActivePetID != 0 {
		query = g.store.DB.Where("id = ? AND player_id = ?", player.ActivePetID, player.ID)
	}
	if query.Order("id DESC").First(&pet).Error == nil {
		relations.Pet = fmt.Sprintf("%s · LV%d · 战力%d", pet.Name, maxInt(pet.Level, 1), petCombatPower(pet))
	}
	return relations
}

func drawImage(destination *image.RGBA, source image.Image) {
	for y := source.Bounds().Min.Y; y < source.Bounds().Max.Y; y++ {
		for x := source.Bounds().Min.X; x < source.Bounds().Max.X; x++ {
			destination.Set(x, y, source.At(x, y))
		}
	}
}

func fillOverlay(destination *image.RGBA, rectangle image.Rectangle, overlay color.RGBA) {
	for y := rectangle.Min.Y; y < rectangle.Max.Y; y++ {
		for x := rectangle.Min.X; x < rectangle.Max.X; x++ {
			base := color.RGBAModel.Convert(destination.At(x, y)).(color.RGBA)
			alpha := uint32(overlay.A)
			inverse := uint32(255 - overlay.A)
			destination.SetRGBA(x, y, color.RGBA{
				R: uint8((uint32(overlay.R)*alpha + uint32(base.R)*inverse) / 255),
				G: uint8((uint32(overlay.G)*alpha + uint32(base.G)*inverse) / 255),
				B: uint8((uint32(overlay.B)*alpha + uint32(base.B)*inverse) / 255),
				A: 255,
			})
		}
	}
}

func drawCircularAvatar(destination *image.RGBA, source image.Image, center image.Point, radius int) {
	bounds := source.Bounds()
	side := minInt(bounds.Dx(), bounds.Dy())
	start := image.Pt(bounds.Min.X+(bounds.Dx()-side)/2, bounds.Min.Y+(bounds.Dy()-side)/2)
	for y := -radius; y < radius; y++ {
		for x := -radius; x < radius; x++ {
			distance := x*x + y*y
			if distance <= radius*radius {
				sourceX := start.X + (x+radius)*side/(radius*2)
				sourceY := start.Y + (y+radius)*side/(radius*2)
				destination.Set(center.X+x, center.Y+y, source.At(sourceX, sourceY))
			}
		}
	}
}

func loadStatusAvatar(value string) (image.Image, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if strings.HasPrefix(strings.ToLower(value), "http://") || strings.HasPrefix(strings.ToLower(value), "https://") {
		client := &http.Client{Timeout: 8 * time.Second}
		request, err := http.NewRequest(http.MethodGet, value, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("User-Agent", "XianChen-StatusCard/1.0")
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("头像接口状态码%d", response.StatusCode)
		}
		return decodeLimitedImage(response.Body)
	}
	file, err := os.Open(value)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return decodeLimitedImage(file)
}

func decodeLimitedImage(reader io.Reader) (image.Image, error) {
	limited := io.LimitReader(reader, 8<<20)
	imageValue, format, err := image.Decode(limited)
	if err != nil {
		return nil, err
	}
	if format != "jpeg" && format != "png" && format != "gif" {
		return nil, fmt.Errorf("不支持的头像格式%s", format)
	}
	return imageValue, nil
}

func drawFittedText(destination *image.RGBA, selectedFont *statusFont, text string, x, baseline, maximumWidth, size, minimumSize int, textColor color.Color) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	fitted, fittedSize, _, height := fittedStatusText(selectedFont, text, maximumWidth, size, minimumSize)
	if fitted == "" {
		return
	}
	_ = paintStatusText(destination, selectedFont, fitted, x+2, baseline-height+2, fittedSize, color.RGBA{A: 170})
	_ = paintStatusText(destination, selectedFont, fitted, x, baseline-height, fittedSize, textColor)
}

func drawCenteredFittedText(destination *image.RGBA, selectedFont *statusFont, text string, centerX, baseline, maximumWidth, size, minimumSize int, textColor color.Color) {
	fitted, fittedSize, width, height := fittedStatusText(selectedFont, strings.TrimSpace(text), maximumWidth, size, minimumSize)
	if fitted == "" {
		return
	}
	_ = paintStatusText(destination, selectedFont, fitted, centerX-width/2, baseline-height, fittedSize, textColor)
}

func fittedStatusText(selectedFont *statusFont, text string, maximumWidth, size, minimumSize int) (string, int, int, int) {
	for current := size; current >= minimumSize; current -= 2 {
		width, height, err := measureStatusText(selectedFont, text, current)
		if err == nil && width <= maximumWidth {
			return text, current, width, height
		}
	}
	runes := []rune(text)
	for len(runes) > 1 {
		runes = runes[:len(runes)-1]
		candidate := string(runes) + "…"
		width, height, err := measureStatusText(selectedFont, candidate, minimumSize)
		if err == nil && width <= maximumWidth {
			return candidate, minimumSize, width, height
		}
	}
	return "", 0, 0, 0
}

func drawStatusRows(destination *image.RGBA, selectedFont *statusFont, firstBaseline int, rows []string, x, maximumWidth, size int, textColor color.Color) {
	for index, row := range rows {
		drawFittedText(destination, selectedFont, row, x, firstBaseline+index*52, maximumWidth, size, 19, textColor)
	}
}

func drawProgressBar(destination *image.RGBA, rectangle image.Rectangle, ratio float64, barColor color.RGBA) {
	fillOverlay(destination, rectangle, color.RGBA{R: 15, G: 24, B: 22, A: 220})
	inner := image.Rect(rectangle.Min.X+2, rectangle.Min.Y+2, rectangle.Max.X-2, rectangle.Max.Y-2)
	width := int(float64(inner.Dx()) * ratio)
	if width > 0 {
		fillOverlay(destination, image.Rect(inner.Min.X, inner.Min.Y, inner.Min.X+width, inner.Max.Y), barColor)
	}
}

func fraction(current, maximum int64) float64 {
	if maximum <= 0 {
		return 0
	}
	value := float64(current) / float64(maximum)
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func statusNumber(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}
	digits := strconv.FormatInt(value, 10)
	for index := len(digits) - 3; index > 0; index -= 3 {
		digits = digits[:index] + "," + digits[index:]
	}
	if negative {
		return "-" + digits
	}
	return digits
}

func compactAccountID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 15 {
		return displayOr(value, "未知")
	}
	return value[:8] + "…" + value[len(value)-5:]
}

func statusState(player *model.Player) string {
	if player.Health <= 1 {
		return "重伤濒危"
	}
	switch strings.TrimSpace(player.State) {
	case "修炼中", "闭关", "cultivating":
		return "闭关入定"
	case "战斗中", "battle", "fighting":
		return "斗法之中"
	case "死亡", "dead":
		return "元神离体"
	case "", "空闲", "idle":
		return "云游待命"
	default:
		return player.State
	}
}

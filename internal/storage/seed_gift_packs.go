package storage

import (
	"encoding/json"
	"fmt"

	"xianlv/internal/model"
)

type seededGiftRewards struct {
	Items        map[string]int64 `json:"items"`
	Artifacts    []string         `json:"artifacts"`
	SpiritStones int64            `json:"spirit_stones"`
	SilverCoins  int64            `json:"silver_coins"`
	ImmortalJade int64            `json:"immortal_jade,omitempty"`
	Cultivation  int64            `json:"cultivation"`
	Merit        int64            `json:"merit"`
	Reputation   int64            `json:"reputation"`
	DaoHeart     int64            `json:"dao_heart,omitempty"`
}

// seedGiftPackCatalog builds one thousand discoverable cultivation gift packs
// for production data packs. Every pack has a non-numeric cultivation name,
// its own medicine and its own artifact; reward items are never reused across
// generated packs.
func (s *Store) seedGiftPackCatalog() error {
	return s.seedGiftPackCatalogLimit(contentSeedLimit())
}

func (s *Store) seedGiftPackCatalogLimit(limit int) error {
	blessings := []string{"问道", "凝真", "筑元", "蕴神", "化虚", "渡厄", "登仙", "证道", "归一", "长生"}
	containers := []string{"天赐礼匣", "山河秘匣", "星河宝匣", "洞天珍函", "云海玉匣", "紫府灵匣", "太虚道匣", "九霄宝函", "万象法匣", "长生秘函"}
	rarityNames := []string{"灵品", "仙品", "神品", "玄阶", "地阶", "天阶", "上品", "极品", "圣品", "鸿蒙"}
	for index := 1; index <= limit; index++ {
		seedName := cultivationSeedName(index)
		medicine := cultivationMedicineProfile(index, seedName)
		artifactName := cultivationArtifactName(index, seedName)
		giftName := seedName + blessings[(index-1)%len(blessings)] + containers[((index-1)/len(blessings))%len(containers)]
		rewards := seededGiftRewards{
			Items:        map[string]int64{medicine.OutputName: int64(1 + index%5)},
			Artifacts:    []string{artifactName},
			SpiritStones: int64(180 + index*17),
			SilverCoins:  int64(60 + index*9),
			Cultivation:  int64(90 + index*23),
			Merit:        int64(1 + index%31),
			Reputation:   int64(2 + index%29),
		}
		if index%10 == 0 {
			rewards.ImmortalJade = int64(1 + index/10)
			rewards.DaoHeart = int64(1 + index%7)
		}
		encoded, err := json.Marshal(rewards)
		if err != nil {
			return err
		}
		pack := model.Item{
			Code:         "cultivation_gift_" + seedName,
			Name:         giftName,
			CategoryName: "礼包",
			RarityName:   rarityNames[(index-1)%len(rarityNames)],
			Description:  fmt.Sprintf("封存%s道统的修行馈赠，内蕴独有丹药%s与本命器胚%s，适合在对应道阶补足修为、声望与护道资源。", seedName, medicine.OutputName, artifactName),
			EffectType:   "修仙礼包",
			EffectFunc:   "open_gift_pack",
			EffectParams: string(encoded),
			BaseValue:    int64(500 + index*73),
			StackLimit:   999,
			Stackable:    true,
			Tradable:     false,
			StoreEnabled: true,
			StorePrice:   int64(200 + index*37),
		}
		if err := s.firstOrCreateCodeName(&pack, pack.Code, pack.Name); err != nil {
			return err
		}
		currency := "银币"
		price := int64(300 + index*41)
		if index%5 == 0 {
			currency = "仙金"
			price = int64(6 + index*3)
		}
		shop := model.ShopEntry{
			Code: "cultivation_gift_shop_" + seedName, ItemID: pack.ID, ItemName: pack.Name,
			Currency: currency, Price: price, PurchaseLimit: 0, RefreshCycle: "永不", Sort: 20000 + index, Enabled: true,
		}
		if err := s.DB.Where("code = ?", shop.Code).FirstOrCreate(&shop).Error; err != nil {
			return err
		}
	}
	return nil
}

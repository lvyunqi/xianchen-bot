package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"xianlv/internal/model"
)

var adminArrayJSONFields = map[string]bool{
	"npc_json": true, "tasks_json": true, "neighbors_json": true,
}

func (a *AdminAPI) prepareResourceCreate(resource string, row any) error {
	value := reflect.ValueOf(row)
	if value.Kind() != reflect.Pointer || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("后台资源类型不正确")
	}
	value = value.Elem()
	if field := value.FieldByName("Code"); field.IsValid() && field.CanSet() && field.Kind() == reflect.String && strings.TrimSpace(field.String()) == "" {
		field.SetString(newAdminResourceCode(resource))
	}
	if field := value.FieldByName("Name"); field.IsValid() && field.Kind() == reflect.String {
		name := strings.TrimSpace(field.String())
		if name == "" {
			return fmt.Errorf("名称不能为空")
		}
		field.SetString(name)
	}
	if err := normalizeAdminStructuredStruct(value); err != nil {
		return err
	}
	switch current := row.(type) {
	case *model.Item:
		return a.prepareAdminItem(current)
	case *model.ShopEntry:
		return a.prepareAdminShopEntry(current)
	case *model.AlchemyRecipe:
		return a.prepareAdminAlchemyRecipe(current)
	case *model.SynthesisRecipe:
		return a.prepareAdminSynthesisRecipe(current)
	case *model.ArtifactTemplate:
		if strings.TrimSpace(current.Archetype) == "" {
			current.Archetype = strings.TrimSpace(current.Type)
		}
		if strings.TrimSpace(current.Type) == "" {
			current.Type = current.Archetype
		}
		if strings.TrimSpace(current.Slot) == "" {
			return fmt.Errorf("请选择真实穿戴槽位")
		}
	case *model.Notice:
		now := time.Now()
		if current.Published {
			current.PublishedAt = &now
		} else {
			current.PublishedAt = nil
		}
	}
	return nil
}

func (a *AdminAPI) prepareResourceUpdate(resource string, changes map[string]any) error {
	for key, raw := range changes {
		if text, ok := raw.(string); ok {
			changes[key] = strings.TrimSpace(text)
		}
		if !isAdminStructuredField(key) {
			continue
		}
		normalized, err := normalizeAdminStructuredValue(key, raw)
		if err != nil {
			return err
		}
		changes[key] = normalized
	}
	if name, exists := changes["name"]; exists && strings.TrimSpace(fmt.Sprint(name)) == "" {
		return fmt.Errorf("名称不能为空")
	}
	if code, exists := changes["code"]; exists && strings.TrimSpace(fmt.Sprint(code)) == "" {
		return fmt.Errorf("系统编码不能清空；新增时可留空自动生成")
	}
	if resource == "notices" {
		published, exists := changes["published"]
		if !exists {
			delete(changes, "published_at")
		} else {
			value, ok := published.(bool)
			if !ok {
				return fmt.Errorf("发布状态必须是true或false")
			}
			if value {
				now := time.Now()
				changes["published_at"] = &now
			} else {
				changes["published_at"] = nil
			}
		}
	}
	if resource == "items" {
		if category, exists := changes["category_name"]; exists {
			var row model.ItemCategory
			if err := a.store.DB.Where("name = ?", strings.TrimSpace(fmt.Sprint(category))).First(&row).Error; err != nil {
				return fmt.Errorf("物品分类不存在，请从下拉选项选择")
			}
			changes["category_id"] = row.ID
		}
		if rarity, exists := changes["rarity_name"]; exists {
			var row model.Rarity
			if err := a.store.DB.Where("name = ?", strings.TrimSpace(fmt.Sprint(rarity))).First(&row).Error; err != nil {
				return fmt.Errorf("稀有度不存在，请从下拉选项选择")
			}
			changes["rarity_id"] = row.ID
		}
		for _, key := range []string{"base_value", "stack_limit", "store_price"} {
			if raw, exists := changes[key]; exists && adminNumber(raw) < 0 {
				return fmt.Errorf("%s不能为负数", adminFieldLabel(key))
			}
		}
	}
	return nil
}

func normalizeAdminStructuredStruct(value reflect.Value) error {
	typeInfo := value.Type()
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		if !field.CanSet() || field.Kind() != reflect.String {
			continue
		}
		jsonName := strings.Split(typeInfo.Field(index).Tag.Get("json"), ",")[0]
		if !isAdminStructuredField(jsonName) {
			continue
		}
		normalized, err := normalizeAdminStructuredValue(jsonName, field.String())
		if err != nil {
			return err
		}
		field.SetString(normalized)
	}
	return nil
}

func isAdminStructuredField(key string) bool {
	return strings.HasSuffix(key, "_json") || key == "effect_params" || key == "cost_materials" || key == "prerequisite" || key == "evolution_condition" || key == "attribute_bonus"
}

func normalizeAdminStructuredValue(key string, raw any) (string, error) {
	if raw == nil || strings.TrimSpace(fmt.Sprint(raw)) == "" {
		if adminArrayJSONFields[key] {
			return "[]", nil
		}
		return "{}", nil
	}
	var encoded []byte
	switch value := raw.(type) {
	case string:
		encoded = []byte(strings.TrimSpace(value))
	default:
		var err error
		encoded, err = json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("%s无法转换为结构化内容", adminFieldLabel(key))
		}
	}
	if !json.Valid(encoded) {
		return "", fmt.Errorf("%s格式不完整，请使用后台的中文项目编辑器", adminFieldLabel(key))
	}
	var compact any
	if err := json.Unmarshal(encoded, &compact); err != nil {
		return "", fmt.Errorf("%s格式错误: %v", adminFieldLabel(key), err)
	}
	encoded, _ = json.Marshal(compact)
	return string(encoded), nil
}

func (a *AdminAPI) prepareAdminItem(item *model.Item) error {
	item.Name = strings.TrimSpace(item.Name)
	if item.CategoryName == "" {
		item.CategoryName = "材料"
	}
	if item.RarityName == "" {
		item.RarityName = "凡品"
	}
	var category model.ItemCategory
	if err := a.store.DB.Where("name = ?", item.CategoryName).First(&category).Error; err != nil {
		return fmt.Errorf("物品分类“%s”不存在，请从下拉选项选择", item.CategoryName)
	}
	var rarity model.Rarity
	if err := a.store.DB.Where("name = ?", item.RarityName).First(&rarity).Error; err != nil {
		return fmt.Errorf("稀有度“%s”不存在，请从下拉选项选择", item.RarityName)
	}
	item.CategoryID, item.RarityID = category.ID, rarity.ID
	if item.StackLimit < 0 || item.BaseValue < 0 || item.StorePrice < 0 {
		return fmt.Errorf("价值、堆叠上限和商城价格不能为负数")
	}
	if item.StackLimit == 0 {
		if item.Stackable {
			item.StackLimit = 999
		} else {
			item.StackLimit = 1
		}
	}
	if item.StoreEnabled && item.StorePrice <= 0 {
		return fmt.Errorf("启用商城出售时，商城价格必须大于0")
	}
	return nil
}

func (a *AdminAPI) prepareAdminShopEntry(entry *model.ShopEntry) error {
	if entry.ItemID == 0 && strings.TrimSpace(entry.ItemName) != "" {
		var item model.Item
		if err := a.store.DB.Where("name = ?", strings.TrimSpace(entry.ItemName)).First(&item).Error; err != nil {
			return fmt.Errorf("没有找到物品“%s”", entry.ItemName)
		}
		entry.ItemID = item.ID
	}
	if entry.ItemID == 0 {
		return fmt.Errorf("请填写物品名")
	}
	var item model.Item
	if err := a.store.DB.First(&item, entry.ItemID).Error; err != nil {
		return fmt.Errorf("商城物品不存在")
	}
	entry.ItemName = item.Name
	if entry.Price <= 0 {
		return fmt.Errorf("商城价格必须大于0")
	}
	return nil
}

func (a *AdminAPI) prepareAdminAlchemyRecipe(recipe *model.AlchemyRecipe) error {
	if strings.TrimSpace(recipe.OutputName) == "" {
		return fmt.Errorf("请填写炼制产物名称")
	}
	var item model.Item
	if err := a.store.DB.Where("name = ?", strings.TrimSpace(recipe.OutputName)).First(&item).Error; err != nil {
		return fmt.Errorf("产物“%s”尚未添加到物品数据", recipe.OutputName)
	}
	recipe.OutputItemID = item.ID
	if recipe.SuccessRate < 0 || recipe.SuccessRate > 1 {
		return fmt.Errorf("成功率应填写0至1之间的小数")
	}
	return nil
}

func (a *AdminAPI) prepareAdminSynthesisRecipe(recipe *model.SynthesisRecipe) error {
	if strings.TrimSpace(recipe.OutputName) == "" {
		return fmt.Errorf("请填写合成产物名称")
	}
	var item model.Item
	if err := a.store.DB.Where("name = ?", strings.TrimSpace(recipe.OutputName)).First(&item).Error; err != nil {
		return fmt.Errorf("产物“%s”尚未添加到物品数据", recipe.OutputName)
	}
	recipe.OutputItemID = item.ID
	if recipe.OutputQuantity <= 0 {
		recipe.OutputQuantity = 1
	}
	if recipe.SuccessRate < 0 || recipe.SuccessRate > 1 {
		return fmt.Errorf("成功率应填写0至1之间的小数")
	}
	return nil
}

func newAdminResourceCode(resource string) string {
	random := make([]byte, 5)
	if _, err := rand.Read(random); err != nil {
		stamp := strconv.FormatInt(time.Now().UnixNano(), 36)
		return "admin_" + strings.ReplaceAll(resource, "-", "_") + "_" + stamp
	}
	return "admin_" + strings.ReplaceAll(resource, "-", "_") + "_" + hex.EncodeToString(random)
}

func adminNumber(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	case int64:
		return float64(number)
	default:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		return parsed
	}
}

func adminFieldLabel(key string) string {
	labels := map[string]string{
		"effect_params": "效果内容", "cost_materials": "消耗材料", "prerequisite": "前置条件",
		"reward_json": "奖励内容", "materials_json": "材料内容", "attribute_json": "属性内容",
		"objective_json": "任务目标", "condition_json": "触发条件", "base_value": "基础价值",
		"stack_limit": "堆叠上限", "store_price": "商城价格",
	}
	if label := labels[key]; label != "" {
		return label
	}
	return key
}

const fallbackPages = [
  { group: '数据总览', key: 'dashboard', title: '仪表盘', icon: '览', description: '玩家、活跃、仙侣、内容与运行趋势' },
  { group: '核心数据', key: 'config', title: '系统参数', icon: '⚙', description: '修炼、渡劫、体力与全局参数' },
  { group: '核心数据', key: 'features', title: '功能开关', icon: '开', description: '开启或关闭任意游戏模块' },
  { group: '核心数据', key: 'constants', title: '游戏常量', icon: '常', description: '等级、转世、背包等全局常量' },
  { group: '核心数据', key: 'cooldowns', title: '冷却时间', icon: '时', description: '各操作冷却时间配置' },
  { group: '核心数据', key: 'realms', title: '境界配置', icon: '山', description: '境界需求、属性成长与寿元' },
  { group: '核心数据', key: 'spiritual_roots', title: '灵根图鉴', icon: '根', description: '一千种灵根、稀有权重与完整加成' },
  { group: '内容配置', key: 'items', title: '物品数据', icon: '物', description: '物品效果、价值、交易与堆叠' },
  { group: '内容配置', key: 'events', title: '事件数据', icon: '缘', description: '随机事件、条件、概率与奖励' },
  { group: '内容配置', key: 'tasks', title: '任务数据', icon: '任', description: '日常、悬赏、宗门与成就任务' },
  { group: '内容配置', key: 'skills', title: '功法数据', icon: '诀', description: '功法类型、效果、升级与条件' },
  { group: '内容配置', key: 'pets', title: '灵兽数据', icon: '兽', description: '灵兽成长、忠诚与进化关系' },
  { group: '内容配置', key: 'dungeons', title: '副本数据', icon: '境', description: '副本难度、战力、体力与奖励池' },
  { group: '内容配置', key: 'recipes', title: '丹方数据', icon: '丹', description: '丹方材料、产物和成功率' },
  { group: '内容配置', key: 'artifacts', title: '器谱数据', icon: '器', description: '法宝材料、属性和强化上限' },
  { group: '内容配置', key: 'synthesis_recipes', title: '合成配方', icon: '合', description: '材料合成、产物、成功率与前置条件' },
  { group: '内容配置', key: 'locations', title: '地图数据', icon: '图', description: '地点、区域、通行路线、境界条件与体力消耗' },
  { group: '内容配置', key: 'world_leylines', title: '修仙界灵脉', icon: '脉', description: '一千条地域灵脉、阶级、前置与独立加成' },
  { group: '运营数据', key: 'titles', title: '称号数据', icon: '号', description: '称号条件、类型与属性加成' },
  { group: '运营数据', key: 'activities', title: '活动数据', icon: '活', description: '活动时间、效果参数与状态' },
  { group: '运营数据', key: 'mails', title: '邮件数据', icon: '信', description: '邮件正文、奖励、对象与发送' },
  { group: '运营数据', key: 'checkin', title: '签到配置', icon: '签', description: '七日签到奖励与特殊奖励' },
  { group: '运营数据', key: 'shop', title: '商店数据', icon: '店', description: '商品、价格、货币与上架状态，玩家购买不限次数' },
  { group: '运营数据', key: 'cdks', title: '兑换码数据', icon: '码', description: '兑换奖励、次数、期限与状态' },
  { group: '运营数据', key: 'notices', title: '公告数据', icon: '告', description: '公告正文、类型、置顶与发布' },
  { group: '动态数据', key: 'players', title: '玩家数据', icon: '人', description: '查询、编辑、物品、封禁和删除' },
  { group: '动态数据', key: 'couples', title: '仙侣数据', icon: '侣', description: '仙侣关系、道缘与强制操作' },
  { group: '内容审核', key: 'reviews', title: '内容与玩家反馈', icon: '审', description: '审核道号与社交内容，并处理玩家 BUG、玩法建议及自动初审结论' },
  { group: '内容审核', key: 'sensitive_words', title: '敏感词管理', icon: '词', description: '增删改敏感词和替换规则' },
  { group: '系统管理', key: 'menus', title: '菜单管理', icon: '单', description: '后台导航与群内游戏菜单，保存立即生效' },
  { group: '系统管理', key: 'status_display', title: '状态显示', icon: '图', description: '切换状态指令使用纯图片模式或完整文字模式' },
  { group: '系统管理', key: 'owner_settings', title: '主人设置', icon: '主', description: '设置唯一主人QQ开放平台用户ID' },
  { group: '系统管理', key: 'managers', title: '管理设置', icon: '管', description: '管理员ID、名称、权限范围和启用状态' },
  { group: '系统监控', key: 'server_status', title: '服务器状态', icon: '服', description: 'CPU、内存、磁盘和运行时间' },
  { group: '系统监控', key: 'online_monitor', title: '在线监控', icon: '线', description: '最近五分钟实时活跃人数' },
  { group: '系统监控', key: 'request_stats', title: '请求统计', icon: '请', description: '后台 API 请求和错误统计' },
  { group: '系统监控', key: 'slow_queries', title: '慢查询', icon: '慢', description: '数据库慢 SQL 记录' },
  { group: '系统监控', key: 'performance', title: '性能监控', icon: '性', description: '请求耗时和数据库连接状态' },
  { group: '系统监控', key: 'alerts', title: '告警配置', icon: '警', description: 'CPU、内存、磁盘和延迟告警阈值' }
]

// Keep a complete local catalog available while a customized/older menu
// definition is being loaded.  The backend menu is allowed to reorder or
// hide entries, but it must not make a data page unreachable when a stale
// database contains only a partial menu tree.
let pages = [...fallbackPages]

fallbackPages.push(
  ...[
    ['formations','阵法管理','阵'], ['talismans','符箓管理','符'], ['puppets_config','傀儡管理','傀'],
    ['secret_conflicts','秘境争夺','秘'], ['inheritances','传承管理','传'], ['dao_insights','悟道管理','道'],
    ['battlefields','仙魔战场','战'], ['root_evolutions','灵根进化','根'], ['inner_demons','渡劫心魔','魔'],
    ['couple_skills','合体技管理','合'], ['immortal_herbs','仙药培育','药'], ['artifact_refinements','法宝炼化','宝'],
    ['destiny_deductions','天机推演','机'], ['leylines','天地灵脉','脉'], ['sect_wars','宗门战争','宗'],
    ['immortal_encounters','仙缘奇遇','缘'], ['star_realms','宇宙星河','星']
  ].map(([key,title,icon]) => ({ group: '全新玩法', key, title, icon, description: `${title}配置、材料、效果、条件和等级数据` }))
)

const schemas = {
  dashboard: fields(['metric','指标','text'], ['value','当前值','text']),
  config: fields(['key','配置键','text'], ['value','值','text'], ['value_type','类型','select',['int','float','string','bool','json']], ['description','说明','textarea']),
  features: fields(['key','功能键','text'], ['value','开启状态','text'], ['value_type','类型','select',['bool','int','float','string']], ['description','说明','textarea']),
  constants: fields(['key','常量键','text'], ['value','数值','text'], ['value_type','类型','select',['int','float','string','bool']], ['description','说明','textarea']),
  cooldowns: fields(['key','操作键','text'], ['value','冷却秒数','text'], ['value_type','类型','select',['int','float']], ['description','说明','textarea']),
  realms: fields(['name','境界','text'], ['sequence','顺序','number'], ['required_cultivation','需求修为','number'], ['base_health','生命','number'], ['base_mana','法力','number'], ['base_attack','攻击','number'], ['base_defense','防御','number'], ['base_speed','速度','number'], ['base_dodge','闪避率','number'], ['base_lifespan','寿元','number'], ['tribulation_base_rate','渡劫率','number'], ['description','描述','textarea']),
  spiritual_roots: fields(['code','编码','text'], ['name','灵根名称','text'], ['element','本源属性','text'], ['grade','品阶','text'], ['base_quality','基础纯度','number'], ['cultivation_bonus','修炼倍率','number'], ['primary_bonus','主加成','textarea'], ['secondary_bonus','副加成','textarea'], ['combat_description','战斗定位','textarea'], ['description','图鉴描述','textarea'], ['attribute_json','完整属性 JSON','textarea'], ['image_url','图片 URL','text'], ['rarity_weight','随机权重','number'], ['enabled','启用','bool']),
  items: fields(['code','编码','text'], ['name','名称','text'], ['category_name','类型','select',['丹药','材料','灵草','残卷','法宝','礼包','种子','灵肥','嵌灵宝石','生辰','任务物品']], ['rarity_name','稀有度','select',['凡品','灵品','仙品','神品']], ['description','说明','textarea'], ['effect_type','效果类型','select',['材料','修为','治疗比例','法力恢复比例','悟性','神魂','道心','突破','渡劫','转世','复活','修炼','礼包','种植','灵田施肥','灵兽','传送','装备嵌灵','灵根定制','称号定制','法宝定制','仙府定制','灵兽定制']], ['effect_func','效果函数','text'], ['effect_params','效果参数 JSON','textarea'], ['effect_value','效果数值','number'], ['base_value','基础价值','number'], ['stack_limit','堆叠上限','number'], ['stackable','可堆叠','bool'], ['tradable','可交易','bool'], ['store_enabled','商城启用','bool'], ['store_price','商城价格','number'], ['image_url','图片 URL','text']),
  events: fields(['name','名称','text'], ['type','类型','select',['机缘','劫难','奇遇']], ['description','描述','textarea'], ['probability','概率','number'], ['reward_json','奖励 JSON','textarea'], ['condition_json','条件 JSON','textarea'], ['drop_pool_id','掉落池 ID','number'], ['enabled','启用','bool']),
  tasks: fields(['name','名称','text'], ['type','类型','select',['日常','悬赏','宗门','成就','主线','支线']], ['description','描述','textarea'], ['prerequisite_json','前置条件 JSON','textarea'], ['objective_json','目标条件 JSON','textarea'], ['reward_json','奖励 JSON','textarea'], ['weight','权重','number'], ['daily','每日刷新','bool'], ['enabled','启用','bool']),
  skills: fields(['name','名称','text'], ['type','类型','select',['攻击','防御','辅助','均衡']], ['rarity','稀有度','text'], ['realm_required','境界条件','text'], ['description','描述','textarea'], ['effect_json','属性加成 JSON','textarea'], ['upgrade_json','升级配置 JSON','textarea']),
  pets: fields(['code','编码','text'], ['name','名称','text'], ['initial_power','初始战力','number'], ['growth_per_level','每级成长','number'], ['loyalty_decay','忠诚衰减','number'], ['evolution_condition','进化条件 JSON','textarea'], ['evolution_target','进化目标','text'], ['enabled','启用','bool']),
  dungeons: fields(['code','编码','text'], ['name','名称','text'], ['difficulty','难度','select',['普通','困难','噩梦','地狱']], ['recommended_power','推荐战力','number'], ['stamina_cost','体力消耗','number'], ['reward_pool_json','奖励池 JSON','textarea'], ['daily_limit','每日次数','number'], ['enabled','启用','bool'], ['image_url','图片 URL','text']),
  recipes: fields(['code','编码','text'], ['name','名称','text'], ['materials_json','材料 JSON','textarea'], ['output_name','产物','text'], ['success_rate','成功率','number'], ['description','说明','textarea'], ['enabled','启用','bool']),
  artifacts: fields(['code','编码','text'], ['name','名称','text'], ['slot','穿戴槽位','select',['本命法器','冠冕','道袍','护腕','腰佩','灵靴','戒指','项链','护符','阵盘']], ['archetype','器型','text'], ['positioning','战斗定位','text'], ['set_name','套装名称','text'], ['set_bonus_json','套装加成 JSON','textarea'], ['materials_json','炼制材料 JSON','textarea'], ['attribute_json','基础属性 JSON','textarea'], ['minimum_realm_sequence','最低大境','number'], ['minimum_realm_level','最低层数','number'], ['minimum_combat_power','最低战力','number'], ['description','器物说明','textarea'], ['source_json','来源 JSON','textarea'], ['max_level','强化上限','number'], ['enabled','启用','bool']),
  synthesis_recipes: fields(['code','编码','text'], ['name','配方名称','text'], ['category','分类','text'], ['materials_json','材料 JSON','textarea'], ['output_item_id','产物物品 ID','number'], ['output_name','产物名称','text'], ['output_quantity','产物数量','number'], ['success_rate','成功率（0-1）','number'], ['prerequisite_json','前置条件 JSON','textarea'], ['description','配方说明','textarea'], ['enabled','启用','bool'], ['sort_order','排序','number']),
  locations: fields(['code','地点编码','text'], ['name','地点名称','text'], ['region','所属区域','text'], ['description','地点说明','textarea'], ['image_url','图片 URL','text'], ['npc_json','NPC JSON','textarea'], ['tasks_json','地图任务 JSON','textarea'], ['resource_name','区域采集资源','text'], ['resource_quantity','采集数量','number'], ['resource_cooldown_minutes','采集刷新分钟','number'], ['teleport_enabled','传送阵','bool'], ['cross_region_enabled','跨界传送','bool'], ['minimum_realm_sequence','最低境界顺序','number'], ['minimum_realm_level','最低境界层数','number'], ['minimum_level','最低等级','number'], ['stamina_cost','移动体力','number'], ['monster_name','普通怪物','text'], ['monster_power','怪物战力','number'], ['monster_encounter_rate','遭遇概率','number'], ['monster_reward_json','怪物奖励 JSON','textarea'], ['boss_name','区域 Boss','text'], ['boss_power','Boss战力','number'], ['boss_reward_json','Boss奖励 JSON','textarea'], ['boss_cooldown_minutes','Boss冷却分钟','number'], ['neighbors_json','相邻地点 JSON','textarea'], ['enabled','启用','bool'], ['sort_order','排序','number']),
  world_leylines: fields(['code','编码','text'], ['name','灵脉名称','text'], ['region','所属界域','text'], ['location_name','所在地图','text'], ['element','本源属性','text'], ['grade','灵脉阶级','text'], ['aura_per_minute','每分钟灵气','number'], ['cultivation_multiplier','修炼倍率','number'], ['meditation_slots','打坐位','number'], ['discovery_mana_cost','探查法力','number'], ['minimum_realm_sequence','最低境界顺序','number'], ['minimum_realm_level','最低境界层数','number'], ['minimum_combat_power','最低战力','number'], ['minimum_spirit','最低神识','number'], ['required_root_element','契合灵根','text'], ['required_item','护脉材料','text'], ['required_item_count','材料数量','number'], ['bonus_json','独立加成 JSON','textarea'], ['description','详细说明','textarea'], ['image_url','图片 URL','text'], ['enabled','启用','bool'], ['sort_order','排序','number']),
  arena_tiers: fields(['code','编码','text'], ['name','段位名称','text'], ['sequence','段位顺序','number'], ['minimum_rating','晋阶积分','number'], ['daily_coin','每日竞技币','number'], ['daily_silver','每日银币','number'], ['description','段位道意','textarea'], ['enabled','启用','bool']),
  titles: fields(['code','编码','text'], ['name','称号名','text'], ['condition','获取条件','textarea'], ['attribute_bonus','属性加成 JSON','textarea'], ['type','类型','text'], ['enabled','启用','bool']),
  activities: fields(['code','编码','text'], ['name','活动名','text'], ['type','类型','select',['修炼','探索','战斗','渡劫']], ['starts_at','开始时间','datetime-local'], ['ends_at','结束时间','datetime-local'], ['effect','效果','textarea'], ['effect_json','效果参数 JSON','textarea'], ['status','状态','select',['未开始','进行中','已结束']]),
  mails: fields(['code','编码','text'], ['title','标题','text'], ['content','正文','textarea'], ['sender','发件人','text'], ['reward_json','奖励 JSON','textarea'], ['target_type','发送对象','select',['全部','指定玩家']], ['target_id','目标玩家','text'], ['expires_at','过期时间','datetime-local'], ['sent','已发送','bool']),
  checkin: fields(['day','天数','number'], ['item_name','奖励物品','text'], ['quantity','奖励数量','number'], ['special_reward','特殊奖励','text']),
  shop: fields(['code','编码','text'], ['item_id','物品 ID','number'], ['item_name','物品名','text'], ['currency','价格类型','select',['灵石','银币','仙金','贡献','竞技币']], ['price','价格','number'], ['sort','排序','number'], ['enabled','启用','bool']),
  cdks: fields(['code','兑换码','text'], ['reward_json','奖励 JSON','textarea'], ['max_uses','最大次数','number'], ['used_count','已使用','number'], ['expires_at','过期时间','datetime-local'], ['status','状态','select',['有效','已过期','已用完']]),
  notices: fields(['code','编码','text'], ['title','标题','text'], ['content','正文','textarea'], ['type','发布频道','select',['公告','更新','修复','全区通报']], ['pinned','置顶','bool'], ['published','已发布','bool']),
  players: fields(['account_id','账号','text'], ['dao_name','道号','text'], ['avatar_url','头像 URL','text'], ['gender','性别','text'], ['realm_name','境界','text'], ['realm_level','境界层数','number'], ['cultivation','修为','number'], ['cultivation_required','所需修为','number'], ['spiritual_root','灵根','text'], ['root_quality','资质','number'], ['health','气血','number'], ['max_health','气血上限','number'], ['mana','法力','number'], ['max_mana','法力上限','number'], ['physical_attack','物攻','number'], ['magic_attack','法强','number'], ['physical_defense','物抗','number'], ['magic_defense','法抗','number'], ['agility','身法','number'], ['spirit','神识','number'], ['perception','悟性','number'], ['luck','气运','number'], ['combat_power','战力','number'], ['spirit_stones','灵石','number'], ['silver_coins','银币','number'], ['immortal_jade','仙金','number'], ['arena_coins','竞技币','number'], ['merit','功德','number'], ['reputation','声望','number'], ['dao_heart','道心','number'], ['immortal_affinity','仙缘','number'], ['sect_name','宗门','text'], ['location','位置','text'], ['title','称号','text'], ['state','状态','text'], ['banned','封禁','bool'], ['ban_reason','封禁原因','textarea']),
  couples: fields(['player_a_id','玩家 A ID','number'], ['player_b_id','玩家 B ID','number'], ['player_a_name','玩家 A','text'], ['player_b_name','玩家 B','text'], ['affinity','道缘深度','number'], ['bond_level','同心等级','number'], ['cultivation_bonus','修炼加成','number'], ['joint_attack_bonus','合击加成','number'], ['status','状态','text'], ['notes','备注','textarea']),
  menus: fields(['parent_id','父菜单 ID','number'], ['menu_type','菜单类型','select',['side','top','both']], ['label','菜单名称','text'], ['icon','图标文字','text'], ['path','路由/游戏指令','text'], ['component','页面组件','text'], ['permission','使用范围','select',['admin','player','viewer']], ['sort_order','排序值','number'], ['is_hidden','隐藏','bool'], ['is_external','外部链接','bool'], ['external_url','外部链接地址','text'], ['target','打开方式','select',['_self','_blank']], ['badge_type','角标类型','select',['','dot','count']], ['badge_value','角标数值','number'], ['status','状态','select',['active','inactive']])
  ,reviews: fields(['type','内容类型','select',['道号','日记','三生石','传音','留言','灵根定制','称号定制','仙府定制','灵兽定制','法宝定制','BUG反馈','玩法建议']], ['player_id','玩家 ID','number'], ['player_name','玩家道号','text'], ['content','待审内容或玩家反馈','textarea'], ['status','处理状态','select',['待审核','处理中','已通过','已拒绝','已修复']], ['reason','自动初审/人工审核说明','textarea'], ['diagnosis','诊断结果','textarea'], ['resolution_type','处理方式','select',['','自动数据修复','执行链排查','人工排查','玩法评审','人工评审','人工修复','退回补充','自动驳回']], ['resolution','处理结果','textarea'], ['reviewed_at','审核时间','datetime-local'], ['resolved_at','完成时间','datetime-local'])
  ,sensitive_words: fields(['word','敏感词','text'], ['replacement','替换内容','text'], ['enabled','启用','bool'])
  ,server_status: fields(['metric','服务器指标','text'], ['value','当前值','text'])
  ,online_monitor: fields(['metric','在线指标','text'], ['value','当前值','text'])
  ,request_stats: fields(['metric','请求指标','text'], ['value','当前值','text'])
  ,slow_queries: fields(['sql','SQL','textarea'], ['duration_ms','耗时(ms)','number'], ['source','来源','text'], ['created_at','发生时间','text'])
  ,performance: fields(['metric','性能指标','text'], ['value','当前值','text'])
  ,alerts: fields(['key','告警项','text'], ['value','阈值','text'], ['value_type','类型','select',['int','float','bool']], ['description','说明','textarea'])
  ,status_display: fields(['key','配置项','text'], ['value','图片模式（关闭后使用文字）','bool'], ['value_type','类型','select',['bool']], ['description','说明','textarea'])
  ,owner_settings: fields(['key','主人配置项','text'], ['value','QQ开放平台用户ID','text'], ['value_type','类型','select',['string']], ['description','说明','textarea'])
  ,managers: fields(['user_id','管理员用户ID','text'], ['name','管理员名称','text'], ['role','权限层级','select',['护法','长老','宗主','仙尊','道祖']], ['permissions','权限备注','textarea'], ['enabled','启用','bool'])
}

const gameplayFields = () => fields(
  ['code','系统编码','system'], ['name','名称','text'], ['type','类型','select',['攻击','防御','辅助','成长']],
  ['level','等级','number'], ['description','描述','textarea'], ['effect_params','实际效果','structured'],
  ['cost_materials','消耗材料','structured'], ['prerequisite','解锁条件','structured'],
  ['sort_order','排序','number'], ['status','状态','select',['启用','停用']]
)
for (const key of ['formations','talismans','puppets_config','secret_conflicts','inheritances','dao_insights','battlefields','root_evolutions','inner_demons','couple_skills','immortal_herbs','artifact_refinements','destiny_deductions','leylines','sect_wars','immortal_encounters','star_realms']) {
  schemas[key] = gameplayFields()
}

const structuredFieldKeys = new Set([
  'attribute_json','reward_json','condition_json','prerequisite_json','objective_json','effect_json','upgrade_json',
  'evolution_condition','reward_pool_json','materials_json','set_bonus_json','source_json','npc_json','tasks_json',
  'monster_reward_json','boss_reward_json','neighbors_json','bonus_json','attribute_bonus','effect_params','cost_materials','prerequisite'
])
const structuredFieldLabels = {
  attribute_json: '属性项目', reward_json: '奖励项目', condition_json: '触发条件', prerequisite_json: '解锁条件',
  objective_json: '任务目标', effect_json: '实际效果', upgrade_json: '升级规则', evolution_condition: '进化条件',
  reward_pool_json: '副本奖励', materials_json: '所需材料', set_bonus_json: '套装加成', source_json: '获取来源',
  npc_json: '地图人物', tasks_json: '地图任务', monster_reward_json: '妖兽奖励', boss_reward_json: '首领奖励',
  neighbors_json: '相邻地点', bonus_json: '独立加成', attribute_bonus: '称号属性', effect_params: '实际效果',
  cost_materials: '消耗材料', prerequisite: '解锁条件'
}
for (const schema of Object.values(schemas)) {
  for (const field of schema) {
    if (structuredFieldKeys.has(field.key)) {
      field.type = 'structured'
      field.label = structuredFieldLabels[field.key] || field.label.replace(/\s*JSON/ig, '')
    }
    if (field.key === 'code' && schema !== schemas.cdks) {
      field.type = 'system'
      field.label = '系统编码'
    }
  }
}
const itemEffectFunction = schemas.items.find(field => field.key === 'effect_func')
if (itemEffectFunction) {
  itemEffectFunction.type = 'select'
  itemEffectFunction.label = '使用后效果'
  itemEffectFunction.options = [
    { value: '', label: '无主动效果（普通材料）' }, { value: 'add_cultivation', label: '增加修为' },
    { value: 'heal_hp', label: '恢复气血' }, { value: 'restore_mana', label: '恢复法力' },
    { value: 'add_spirit', label: '增加神识' }, { value: 'add_perception', label: '增加悟性' },
    { value: 'add_lifespan', label: '增加寿元' }, { value: 'temporary_buff', label: '限时属性增益' },
    { value: 'breakthrough_bonus', label: '突破成功率增益' }, { value: 'tribulation_bonus', label: '渡劫成功率增益' },
    { value: 'root_refine', label: '灵根淬炼' }, { value: 'revive', label: '复生' },
    { value: 'open_gift_pack', label: '开启礼包' }, { value: 'plant_seed', label: '灵田种子' },
    { value: 'fertilize_crop', label: '灵田施肥' }, { value: 'pet_loyalty', label: '灵兽喂养' },
    { value: 'teleport', label: '传送符' }, { value: 'equipment_gem', label: '装备宝石' },
    { value: 'breakthrough_material', label: '突破材料' }, { value: 'rebirth_guard', label: '转世护持' },
    { value: 'tribulation_guard', label: '付费护劫' }, { value: 'customize_root', label: '定制灵根凭证' },
    { value: 'customize_title', label: '定制称号凭证' }, { value: 'customize_artifact', label: '定制法宝凭证' },
    { value: 'customize_mansion', label: '定制仙府凭证' }, { value: 'customize_pet', label: '定制灵兽凭证' }
  ]
}
for (const key of ['item_id','output_item_id']) {
  for (const schema of Object.values(schemas)) {
    const field = schema.find(item => item.key === key)
    if (field) field.type = 'hidden'
  }
}

function fields(...definitions) {
  return definitions.map(([key, label, type, options]) => ({ key, label, type, options }))
}

const resourceAliases = {
  sensitive_words: 'sensitive-words', slow_queries: 'slow-queries', spiritual_roots: 'spiritual-roots',
  world_leylines: 'world-leylines', arena_tiers: 'arena-tiers', puppets_config: 'puppets-config',
  secret_conflicts: 'secret-conflicts', dao_insights: 'dao-insights', root_evolutions: 'root-evolutions',
  inner_demons: 'inner-demons', couple_skills: 'couple-skills', immortal_herbs: 'immortal-herbs',
  artifact_refinements: 'artifact-refinements', destiny_deductions: 'destiny-deductions',
  sect_wars: 'sect-wars', immortal_encounters: 'immortal-encounters', star_realms: 'star-realms',
  synthesis_recipes: 'synthesis-recipes'
}

function resourcePath(key) { return resourceAliases[key] || key }
function isConfigPage(key) { return ['config','features','constants','cooldowns','alerts','status_display','owner_settings'].includes(key) }
function isReadOnlyPage(key) { return ['dashboard','server_status','online_monitor','request_stats','slow_queries','performance'].includes(key) }
function flattenMetrics(value, prefix = '') {
  const rows = []
  for (const [key, current] of Object.entries(value || {})) {
    const label = prefix ? `${prefix}.${key}` : key
    if (current && typeof current === 'object' && !Array.isArray(current)) rows.push(...flattenMetrics(current, label))
    else rows.push({ metric: label, value: typeof current === 'number' ? Number(current.toFixed?.(2) ?? current) : displayValue(current) })
  }
  return rows
}

const state = { pageKey: 'config', rows: [], page: 1, size: 20, total: 0, editing: null, menuTree: [], searchTimer: null, toastTimer: null }
const $ = selector => document.querySelector(selector)

function renderNavigation() {
  if (state.menuTree.length) {
    const dynamicMarkup = state.menuTree.map(node => renderMenuNode(node, 0)).join('')
    const dynamicKeys = new Set()
    const collectKeys = nodes => (nodes || []).forEach(node => {
      if (node.component && schemas[node.component] && !node.is_hidden && node.status === 'active') dynamicKeys.add(node.component)
      collectKeys(node.children)
    })
    collectKeys(state.menuTree)
    // A menu edited by an older version may contain only one or two entries.
    // Append the missing built-in pages so every data type remains readable.
    const missing = pages.filter(page => !dynamicKeys.has(page.key))
    const fallbackMarkup = missing.length
      ? `<div class="nav-group">完整数据目录</div>${missing.map(page => `<button class="nav-item" data-page="${escapeHTML(page.key)}"><span class="nav-icon">${escapeHTML(page.icon)}</span>${escapeHTML(page.title)}</button>`).join('')}`
      : ''
    $('#navigation').innerHTML = dynamicMarkup + fallbackMarkup
  } else {
    let currentGroup = ''
    $('#navigation').innerHTML = pages.map(page => {
      const heading = page.group !== currentGroup ? `<div class="nav-group">${escapeHTML(page.group)}</div>` : ''
      currentGroup = page.group
      return `${heading}<button class="nav-item" data-page="${page.key}"><span class="nav-icon">${page.icon}</span>${escapeHTML(page.title)}</button>`
    }).join('')
  }
  document.querySelectorAll('.nav-item[data-page]').forEach(button => button.addEventListener('click', () => switchPage(button.dataset.page)))
}

function renderMenuNode(node, depth) {
  if (node.is_hidden || node.status !== 'active') return ''
  const children = (node.children || []).map(child => renderMenuNode(child, depth + 1)).join('')
  const pageKey = node.component && schemas[node.component] ? node.component : ''
  const badge = node.badge_type === 'count' ? `<span class="nav-badge">${Number(node.badge_value || 0)}</span>` : (node.badge_type === 'dot' ? '<span class="nav-dot"></span>' : '')
  if (node.is_external && node.external_url) {
    return `<a class="nav-item nav-link" style="--depth:${depth}" href="${escapeHTML(node.external_url)}" target="${escapeHTML(node.target || '_self')}"><span class="nav-icon">${escapeHTML(node.icon || '链')}</span>${escapeHTML(node.label)}${badge}</a>${children}`
  }
  const own = pageKey
    ? `<button class="nav-item" style="--depth:${depth}" data-page="${escapeHTML(pageKey)}"><span class="nav-icon">${escapeHTML(node.icon || '令')}</span>${escapeHTML(node.label)}${badge}</button>`
    : `<div class="nav-group">${escapeHTML(node.label)}</div>`
  return own + children
}

async function loadNavigation() {
  try {
    const response = await api('/api/menus?type=side')
    state.menuTree = response.data || []
    const dynamicPages = []
    const visit = (nodes, group = '') => (nodes || []).forEach(node => {
      const nextGroup = node.component ? group : node.label
      if (node.component && schemas[node.component]) {
        const fallback = fallbackPages.find(item => item.key === node.component) || {}
        dynamicPages.push({ group: group || '自定义菜单', key: node.component, title: node.label, icon: node.icon || fallback.icon || '令', description: fallback.description || `${node.label}数据管理` })
      }
      visit(node.children, nextGroup)
    })
    visit(state.menuTree)
    // Preserve every local page and let the backend definition override its
    // label/group/icon.  This is important for installs upgraded from a
    // database that predates the newer admin resources.
    const overrides = new Map(dynamicPages.map(page => [page.key, page]))
    pages = fallbackPages.map(page => overrides.get(page.key) || page)
    for (const page of dynamicPages) {
      if (!fallbackPages.some(item => item.key === page.key)) pages.push(page)
    }
  } catch (error) {
    state.menuTree = []
    pages = [...fallbackPages]
    showToast(`菜单读取失败，已使用本地导航：${error.message}`, true)
  }
  renderNavigation()
}

async function switchPage(key) {
  const resourceChanged = state.pageKey !== key
  if (resourceChanged) {
    clearTimeout(state.searchTimer)
    $('#searchInput').value = ''
  }
  state.pageKey = key; state.page = 1; state.editing = null
  const page = pages.find(item => item.key === key) || fallbackPages.find(item => item.key === key)
  if (!page) return
  $('#pageTitle').textContent = page.title
  $('#pageDescription').textContent = page.description
  document.querySelectorAll('.nav-item').forEach(item => item.classList.toggle('active', item.dataset.page === key))
  $('#sidebar').classList.remove('open')
  $('#addButton').textContent = key === 'couples' ? '＋ 强制结缘' : '＋ 新增'
  $('#addButton').hidden = key === 'players' || key === 'status_display' || isReadOnlyPage(key)
  $('#dashboardQuickActions').hidden = key !== 'dashboard'
  $('#noticeFilters').hidden = key !== 'notices'
  $('#searchInput').placeholder = key === 'notices' ? '搜索公告编码、标题或正文' : '搜索名称、编码或说明'
  await loadRows()
}

async function loadRows() {
  const keyword = $('#searchInput').value.trim()
  try {
    if (state.pageKey === 'dashboard') {
      const response = await api('/api/dashboard')
      const data = response.data || {}; const metrics = data.metrics || {}
      state.rows = [
        { metric: '总玩家数', value: metrics.players || 0 }, { metric: '今日活跃', value: metrics.active_today || 0 },
        { metric: '仙侣对数', value: metrics.couples || 0 }, { metric: '总物品数', value: metrics.items || 0 },
        { metric: '待审核内容', value: metrics.pending_reviews || 0 },
        ...(data.trend || []).map(item => ({ metric: `玩家增长 ${item.day}`, value: item.count })),
        ...(data.recent || []).map(item => ({ metric: `最近动态 ${dateTimeLocal(item.created_at).replace('T', ' ')}`, value: `${item.action || '操作'} ${item.target_type || ''} ${item.target_id || ''}`.trim() }))
      ]
      state.total = state.rows.length; renderTable(); return
    }
    if (['server_status','online_monitor','request_stats','performance'].includes(state.pageKey)) {
      const response = await api('/api/monitor')
      const section = { server_status: 'server', online_monitor: 'online', request_stats: 'requests', performance: 'performance' }[state.pageKey]
      state.rows = flattenMetrics(response.data?.[section] || {})
      state.total = state.rows.length; renderTable(); return
    }
    const configPrefix = { features: 'feature.', constants: 'constant.', cooldowns: 'cooldown.', alerts: 'alert.', status_display: 'display.status_', owner_settings: 'owner.' }[state.pageKey]
    const configQuery = new URLSearchParams({ page: String(state.page), size: String(state.size), keyword })
    let url = state.pageKey === 'config'
      ? `/api/config?${configQuery.toString()}`
      : configPrefix
        ? `/api/config?prefix=${encodeURIComponent(configPrefix)}&${configQuery.toString()}`
        : state.pageKey === 'menus'
          ? `/api/menus/list?page=${state.page}&size=${state.size}&keyword=${encodeURIComponent(keyword)}`
          : `/api/${resourcePath(state.pageKey)}?page=${state.page}&size=${state.size}&keyword=${encodeURIComponent(keyword)}`
    if (state.pageKey === 'notices') {
      const params = new URLSearchParams(url.split('?')[1] || '')
      const type = $('#noticeTypeFilter').value
      const published = $('#noticePublishedFilter').value
      if (type) params.set('type', type)
      if (published) params.set('published', published)
      url = `/api/notices?${params.toString()}`
    }
    const response = await api(url)
    if (state.pageKey === 'config' || configPrefix) {
      const payload = response.data || {}
      // Paged servers return listResponse; retain compatibility with an older
      // array response while an upgraded DLL is being swapped into Bee.
      if (Array.isArray(payload)) {
        state.rows = payload; state.total = payload.length
      } else {
        state.rows = payload.items || []; state.total = payload.total || 0
      }
      const pagesCount = Math.max(1, Math.ceil(state.total / state.size))
      if (state.page > pagesCount) { state.page = pagesCount; return loadRows() }
    } else {
      const payload = response.data || {}; state.rows = payload.items || []; state.total = payload.total || 0
      const pagesCount = Math.max(1, Math.ceil(state.total / state.size))
      if (state.page > pagesCount) { state.page = pagesCount; return loadRows() }
    }
    renderTable()
  } catch (error) { showToast(error.message, true) }
}

function renderTable() {
  const schema = schemas[state.pageKey] || []
  const columns = schema.slice(0, state.pageKey === 'players' ? 7 : 6)
  const editable = !isReadOnlyPage(state.pageKey)
  $('#tableHead').innerHTML = `<tr><th>#</th>${columns.map(field => `<th>${escapeHTML(field.label)}</th>`).join('')}<th>操作</th></tr>`
  $('#tableBody').innerHTML = state.rows.map((row, index) => `<tr>
    <td>${(state.page - 1) * state.size + index + 1}</td>
    ${columns.map(field => tableCell(row, field)).join('')}
    <td><div class="row-actions">${state.pageKey === 'notices' ? `<button data-view-notice="${index}">查看全文</button>` : ''}${editable ? `<button data-edit="${index}">编辑</button>${actionButton(row)}${isConfigPage(state.pageKey) ? '' : `<button class="danger" data-delete="${index}">删除</button>`}` : '只读监控'}</div></td>
  </tr>`).join('')
  $('#emptyState').hidden = state.rows.length !== 0
  const pagesCount = Math.max(1, Math.ceil(state.total / state.size))
  $('#totalLabel').textContent = `共 ${state.total} 条`
  const rangeStart = state.total ? (state.page - 1) * state.size + 1 : 0
  const rangeEnd = state.total ? Math.min(state.page * state.size, state.total) : 0
  $('#rangeLabel').textContent = `当前 ${rangeStart}-${rangeEnd} 条`
  $('#pageLabel').textContent = `${state.page} / ${pagesCount}`
  $('#pageSizeSelect').value = String(state.size)
  $('#pageJumpInput').value = String(state.page)
  $('#pageJumpInput').max = String(pagesCount)
  $('#prevButton').disabled = state.page <= 1
  $('#nextButton').disabled = state.page >= pagesCount
  $('#pageJumpButton').disabled = pagesCount <= 1
  document.querySelectorAll('[data-edit]').forEach(button => button.addEventListener('click', () => openEditor(state.rows[Number(button.dataset.edit)])))
  document.querySelectorAll('[data-delete]').forEach(button => button.addEventListener('click', () => deleteRow(state.rows[Number(button.dataset.delete)])))
  document.querySelectorAll('[data-view-notice]').forEach(button => button.addEventListener('click', () => openNoticeViewer(state.rows[Number(button.dataset.viewNotice)])))
  document.querySelectorAll('[data-action]').forEach(button => button.addEventListener('click', () => runAction(button.dataset.action, button.dataset.id)))
}

function tableCell(row, field) {
  const isNoticeContent = state.pageKey === 'notices' && field.key === 'content'
  const className = isNoticeContent ? ' class="notice-content-cell"' : ''
  const title = isNoticeContent ? '' : ` title="${escapeHTML(displayValue(row[field.key]))}"`
  return `<td${className}${title}>${formatCell(row[field.key], field)}</td>`
}

function openNoticeViewer(row) {
  if (!row) return
  $('#noticeViewerTitle').textContent = row.title || '未命名公告'
  const status = booleanValue(row.published) ? '已发布' : '未发布'
  const details = [row.type || '公告', status, booleanValue(row.pinned) ? '已置顶' : '', formatNoticeTime(row.published_at || row.updated_at || row.created_at)].filter(Boolean)
  $('#noticeViewerMeta').innerHTML = details.map((item, index) => `<span class="${index === 1 ? (status === '已发布' ? 'published' : 'draft') : ''}">${escapeHTML(item)}</span>`).join('')
  $('#noticeViewerContent').textContent = row.content || '（正文为空）'
  $('#noticeViewerDialog').showModal()
}

function formatNoticeTime(value) {
  const local = dateTimeLocal(value)
  return local ? `时间 ${local.replace('T', ' ')}` : ''
}

function actionButton(row) {
  if (state.pageKey === 'mails' && !row.sent) return `<button data-action="send" data-id="${row.id}">发送</button>`
  if (state.pageKey === 'notices' && !row.published) return `<button data-action="publish" data-id="${row.id}">发布</button>`
  if (state.pageKey === 'players') return row.banned ? `<button data-action="unban" data-id="${escapeHTML(row.account_id)}">解禁</button>` : `<button data-action="ban" data-id="${escapeHTML(row.account_id)}">封禁</button>`
  if (state.pageKey === 'menus') return `<button data-action="menu-hide" data-id="${row.id}">${row.is_hidden ? '显示' : '隐藏'}</button><button data-action="menu-up" data-id="${row.id}">上移</button><button data-action="menu-down" data-id="${row.id}">下移</button>`
  if (state.pageKey === 'reviews') {
    let actions = row.status === '待审核' ? `<button data-action="approve" data-id="${row.id}">通过</button><button class="danger" data-action="reject" data-id="${row.id}">拒绝</button>` : ''
    if (row.type === 'BUG反馈' && !['已修复','已拒绝'].includes(row.status)) actions += `<button data-action="resolve" data-id="${row.id}">标记修复</button>`
    return actions
  }
  return ''
}

function openEditor(row = null) {
  state.editing = row
  const schema = schemas[state.pageKey] || []
  const page = pages.find(item => item.key === state.pageKey) || fallbackPages.find(item => item.key === state.pageKey)
  $('#editorTitle').textContent = row ? `编辑${page?.title || '数据'}` : (state.pageKey === 'couples' ? '强制结缘' : `新增${page?.title || '数据'}`)
  $('#formFields').innerHTML = schema.map(field => fieldHTML(field, row ? row[field.key] : defaultValue(field))).join('')
  bindStructuredEditors()
  $('#editorDialog').showModal()
}

function fieldHTML(field, value) {
  const wide = ['textarea','structured'].includes(field.type) ? ' wide' : ''
  if (field.type === 'hidden') return `<input name="${field.key}" type="hidden" value="${escapeHTML(value ?? 0)}">`
  if (field.type === 'system') {
    if (!state.editing) return `<input name="${field.key}" type="hidden" value=""><div class="field wide system-field"><span>唯一系统编码会在保存时自动生成，无需填写。</span></div>`
    return `<div class="field"><label for="f-${field.key}">${escapeHTML(field.label)}</label><input id="f-${field.key}" name="${field.key}" type="text" value="${escapeHTML(value ?? '')}" readonly><p class="field-help">用于数据关联，已由系统锁定。</p></div>`
  }
  if (field.type === 'bool') return `<div class="field${wide}"><label>${escapeHTML(field.label)}</label><label class="check-field"><input name="${field.key}" type="checkbox" ${booleanValue(value) ? 'checked' : ''}> 启用</label></div>`
  if (field.type === 'select') return `<div class="field${wide}"><label for="f-${field.key}">${escapeHTML(field.label)}</label><select id="f-${field.key}" name="${field.key}">${field.options.map(option => `<option value="${escapeHTML(selectOptionValue(option))}" ${String(value ?? '') === selectOptionValue(option) ? 'selected' : ''}>${escapeHTML(selectOptionLabel(option))}</option>`).join('')}</select></div>`
  if (field.type === 'structured') {
    const raw = normalizedRawJSON(value, field.key)
    return `<div class="field wide structured-field" data-structured-field="${field.key}">
      <label for="f-${field.key}">${escapeHTML(field.label)}</label>
      <textarea class="structured-input" id="f-${field.key}" name="${field.key}" placeholder="每行填写一个项目，例如：物品.灵果 = 2">${escapeHTML(structuredToText(raw))}</textarea>
      <p class="field-help">每行“项目 = 数值”。分组可写“物品.灵果 = 2”，无需括号、引号或代码。</p>
      <details class="advanced-editor"><summary>高级模式：查看原始数据</summary><textarea class="raw-json-editor" data-raw-for="${field.key}" spellcheck="false">${escapeHTML(raw)}</textarea></details>
    </div>`
  }
  if (field.type === 'textarea') return `<div class="field wide"><label for="f-${field.key}">${escapeHTML(field.label)}</label><textarea id="f-${field.key}" name="${field.key}">${escapeHTML(value ?? '')}</textarea></div>`
  const formatted = field.type === 'datetime-local' ? dateTimeLocal(value) : (value ?? '')
  return `<div class="field${wide}"><label for="f-${field.key}">${escapeHTML(field.label)}</label><input id="f-${field.key}" name="${field.key}" type="${field.type}" step="any" value="${escapeHTML(formatted)}"></div>`
}

async function saveEditor(event) {
  event.preventDefault()
  const saveButton = $('#saveButton')
  saveButton.disabled = true
  saveButton.textContent = '正在保存…'
  try {
    const form = new FormData($('#editorForm'))
    const payload = {}
    for (const field of schemas[state.pageKey] || []) {
      if (field.type === 'bool') payload[field.key] = form.has(field.key)
      else if (field.type === 'number') payload[field.key] = Number(form.get(field.key) || 0)
      else if (field.type === 'datetime-local') payload[field.key] = form.get(field.key) ? new Date(form.get(field.key)).toISOString() : null
      else if (field.type === 'structured') {
        const rawEditor = document.querySelector(`[data-raw-for="${field.key}"]`)
        if (rawEditor?.dataset.dirty === 'true') payload[field.key] = compactRawJSON(rawEditor.value, field.label)
        else payload[field.key] = structuredTextToJSON(form.get(field.key) || '', field.key, field.label)
      }
      else payload[field.key] = form.get(field.key) || ''
    }
    if (isConfigPage(state.pageKey)) {
      const key = state.editing?.key || payload.key
      await api(`/api/config/${encodeURIComponent(key)}`, { method: 'PUT', body: JSON.stringify(payload) })
    } else if (state.pageKey === 'couples' && !state.editing) {
      await api('/api/couples/force', { method: 'POST', body: JSON.stringify(payload) })
    } else if (state.pageKey === 'realms' && !state.editing) {
      await api('/api/realms', { method: 'POST', body: JSON.stringify(payload) })
    } else {
      const id = state.pageKey === 'players' ? state.editing?.account_id : state.editing?.id
      const resource = resourcePath(state.pageKey)
      const url = id ? `/api/${resource}/${encodeURIComponent(id)}` : `/api/${resource}`
      await api(url, { method: id ? 'PUT' : 'POST', body: JSON.stringify(payload) })
    }
    await loadRows()
    if (state.pageKey === 'menus') await loadNavigation()
    $('#editorDialog').close(); showToast('保存成功，已重新读取数据库')
  } catch (error) { showToast(`保存失败：${error.message}`, true) }
  finally { saveButton.disabled = false; saveButton.textContent = '保存' }
}

async function deleteRow(row) {
  const name = row.name || row.title || row.label || row.dao_name || row.code || row.id
  if (!confirm(`确定删除“${name}”吗？此操作不可撤销。`)) return
  const id = state.pageKey === 'players' ? row.account_id : row.id
  try { await api(`/api/${resourcePath(state.pageKey)}/${encodeURIComponent(id)}`, { method: 'DELETE' }); showToast('删除成功'); await loadRows(); if (state.pageKey === 'menus') await loadNavigation() }
  catch (error) { showToast(error.message, true) }
}

async function runAction(action, id) {
  try {
    if (state.pageKey === 'menus') {
      const row = state.rows.find(item => String(item.id) === String(id))
      if (!row) throw new Error('菜单数据已刷新，请重试')
      if (action === 'menu-hide') await api(`/api/menus/${id}/hide`, { method: 'PUT', body: JSON.stringify({ is_hidden: !row.is_hidden }) })
      else await api(`/api/menus/${id}/move`, { method: 'PUT', body: JSON.stringify({ target_parent_id: row.parent_id || 0, target_position: Math.max(0, Number(row.sort_order || 0) + (action === 'menu-up' ? -15 : 15)) }) })
    } else if (state.pageKey === 'players') await api(`/api/players/${encodeURIComponent(id)}/${action}`, { method: 'POST', body: JSON.stringify({ reason: '后台操作' }) })
    else await api(`/api/${resourcePath(state.pageKey)}/${id}/${action}`, { method: 'POST' })
    showToast('操作成功'); await loadRows(); if (state.pageKey === 'menus') await loadNavigation()
  } catch (error) { showToast(error.message, true) }
}

async function importJSON(file) {
  const body = new FormData(); body.append('file', file)
  const url = state.pageKey === 'menus' ? '/api/menus/import' : `/api/import/${resourcePath(state.pageKey)}`
  try { await api(url, { method: 'POST', body }); showToast('导入完成'); await loadRows(); if (state.pageKey === 'menus') await loadNavigation() }
  catch (error) { showToast(error.message, true) }
}

async function importExcel(file) {
  const body = new FormData(); body.append('file', file)
  try { await api(`/api/import/${resourcePath(state.pageKey)}`, { method: 'POST', body }); showToast('Excel 导入完成'); await loadRows() }
  catch (error) { showToast(error.message, true) }
}

async function uploadImage(file) {
  const body = new FormData(); body.append('file', file)
  try { const result = await api('/api/upload', { method: 'POST', body }); showToast(`图片上传成功：${result.data.url}`) }
  catch (error) { showToast(error.message, true) }
}

async function api(url, options = {}) {
  const headers = options.body instanceof FormData ? {} : { 'Content-Type': 'application/json; charset=utf-8' }
  const controller = options.signal ? null : new AbortController()
  const timeout = controller ? setTimeout(() => controller.abort(), 30000) : null
  try {
    const response = await fetch(url, { ...options, signal: options.signal || controller.signal, headers: { ...headers, ...(options.headers || {}) } })
    const payload = await response.json().catch(() => ({ code: response.status, message: '服务器响应无法解析' }))
    if (!response.ok || payload.code !== 0) throw new Error(payload.message || `请求失败 ${response.status}`)
    return payload
  } catch (error) {
    if (error?.name === 'AbortError') throw new Error('请求超过30秒，服务正在恢复，请稍后重试')
    throw error
  } finally {
    if (timeout) clearTimeout(timeout)
  }
}

function formatCell(value, field) {
  if (field.type === 'bool') return `<span class="boolean ${booleanValue(value) ? 'true' : 'false'}">${booleanValue(value) ? '是' : '否'}</span>`
  if (field.type === 'select') return `<span class="badge">${escapeHTML(displayValue(value))}</span>`
  if (field.type === 'structured') return escapeHTML(structuredSummary(value))
  if (state.pageKey === 'notices' && field.key === 'content') return `<div class="notice-summary">${escapeHTML(noticeSummary(value))}</div>`
  return escapeHTML(displayValue(value))
}

function noticeSummary(value) {
  const text = String(value ?? '').trim()
  if (!text) return '（正文为空）'
  const characters = Array.from(text)
  if (characters.length <= 260) return text
  return `${characters.slice(0, 260).join('').trimEnd()}\n\n（正文共 ${characters.length} 字，请点击“查看全文”继续阅读）`
}

function displayValue(value) {
  if (value === null || value === undefined || value === '') return '—'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function booleanValue(value) {
  if (typeof value === 'boolean') return value
  return ['1','true','yes','on','开启','启用','图片','图片模式'].includes(String(value ?? '').trim().toLowerCase())
}

function defaultValue(field) {
  if (field.type === 'bool') return !new Set(['is_hidden','is_external','store_enabled','published','sent','pinned','banned','cross_region_enabled']).has(field.key)
  if (field.type === 'number') return 0
  if (field.type === 'select') return selectOptionValue(field.options[0])
  if (field.type === 'structured') return adminArrayStructuredFields.has(field.key) ? '[]' : '{}'
  return ''
}

const structuredKeyLabels = {
  items:'物品', artifacts:'装备', cultivation:'修为', merit:'功德', reputation:'声望', spirit_stones:'灵石', silver_coins:'银币',
  immortal_jade:'仙金', arena_coins:'竞技币', title:'称号', attack:'攻击', defense:'防御', health:'气血', mana:'法力', speed:'身法',
  power:'威力', all_percent:'全属性百分比', alchemy_percent:'炼丹成功率百分比', cultivation_percent:'修炼收益百分比',
  max_health_percent:'最大气血恢复百分比', max_mana_percent:'最大法力恢复百分比', cultivation_multiplier:'修炼倍率',
  duration_minutes:'药效分钟', minutes:'持续分钟', rate:'概率', success_rate:'成功率', count:'数量', target:'目标', type:'类型',
  minimum_realm_sequence:'最低大境', minimum_realm_level:'最低层数', minimum_combat_power:'最低战力', minimum_spirit:'最低神识',
  minimum_luck:'最低运气', minimum_merit:'最低功德', minimum_affinity:'最低仙缘', minimum_level:'最低等级', required_item:'所需物品',
  required_item_count:'所需数量', crop:'成熟灵植', grow_minutes:'生长分钟', yield:'基础产量', yield_bonus:'增产数量',
  disaster_resistance:'抗灾值', advance_minutes:'提前分钟', advance_percent:'缩短百分比', region:'界域', location:'地点',
  name:'名称', reward:'奖励', failure:'失败结果', choices:'选择', unique_effect:'独有效果', two:'两件套', four:'四件套', six:'六件套'
}
const structuredLabelKeys = Object.fromEntries(Object.entries(structuredKeyLabels).map(([key, label]) => [label, key]))
const adminArrayStructuredFields = new Set(['npc_json','tasks_json','neighbors_json'])

function selectOptionValue(option) { return String(typeof option === 'object' ? option.value : option) }
function selectOptionLabel(option) { return String(typeof option === 'object' ? option.label : option) }

function normalizedRawJSON(value, fieldKey) {
  if (value && typeof value === 'object') return JSON.stringify(value)
  const text = String(value ?? '').trim()
  if (!text) return adminArrayStructuredFields.has(fieldKey) ? '[]' : '{}'
  try { return JSON.stringify(JSON.parse(text)) } catch { return adminArrayStructuredFields.has(fieldKey) ? '[]' : '{}' }
}

function structuredToText(raw) {
  let value
  try { value = JSON.parse(raw) } catch { return '' }
  const lines = []
  const walk = (current, path = []) => {
    if (Array.isArray(current)) {
      if (current.every(item => item === null || ['string','number','boolean'].includes(typeof item))) {
        lines.push(path.length ? `${formatStructuredPath(path)} = ${current.join('、')}` : current.join('、'))
      } else current.forEach((item, index) => walk(item, [...path, index]))
      return
    }
    if (current && typeof current === 'object') {
      for (const [key, child] of Object.entries(current)) walk(child, [...path, key])
      return
    }
    if (path.length) lines.push(`${formatStructuredPath(path)} = ${current ?? ''}`)
  }
  walk(value)
  return lines.join('\n')
}

function formatStructuredPath(path) {
  return path.map(segment => typeof segment === 'number' ? `第${segment + 1}项` : (structuredKeyLabels[segment] || segment)).join('.')
}

function structuredTextToJSON(text, fieldKey, label) {
  const lines = String(text).split(/\r?\n/).map(line => line.trim()).filter(Boolean)
  if (!lines.length) return adminArrayStructuredFields.has(fieldKey) ? '[]' : '{}'
  const root = adminArrayStructuredFields.has(fieldKey) ? [] : {}
  for (const [index, line] of lines.entries()) {
    const separator = line.search(/[=＝:：]/)
    if (Array.isArray(root) && separator < 0) {
      root.push(...String(line).split('、').map(part => parseStructuredValue(part.trim())))
      continue
    }
    if (separator < 1) throw new Error(`${label}第${index + 1}行缺少“=”`)
    const rawPath = line.slice(0, separator).trim()
    const rawValue = line.slice(separator + 1).trim()
    const path = rawPath.split('.').map(segment => {
      const match = segment.match(/^第(\d+)项$/)
      if (match) return Math.max(0, Number(match[1]) - 1)
      return structuredLabelKeys[segment] || segment
    })
    assignStructuredValue(root, path, parseStructuredValue(rawValue))
  }
  return JSON.stringify(root)
}

function assignStructuredValue(root, path, value) {
  let cursor = root
  path.forEach((segment, index) => {
    const last = index === path.length - 1
    if (last) { cursor[segment] = value; return }
    const nextIsArray = typeof path[index + 1] === 'number'
    if (cursor[segment] === undefined || cursor[segment] === null || typeof cursor[segment] !== 'object') cursor[segment] = nextIsArray ? [] : {}
    cursor = cursor[segment]
  })
}

function parseStructuredValue(value) {
  if (value.includes('、')) return value.split('、').map(part => parseStructuredValue(part.trim()))
  if (/^-?\d+(\.\d+)?$/.test(value)) return Number(value)
  if (['是','开启','启用','true'].includes(value.toLowerCase())) return true
  if (['否','关闭','停用','false'].includes(value.toLowerCase())) return false
  return value
}

function compactRawJSON(value, label) {
  try { return JSON.stringify(JSON.parse(String(value || '{}'))) }
  catch (error) { throw new Error(`${label}高级数据格式错误：${error.message}`) }
}

function structuredSummary(value) {
  const text = structuredToText(normalizedRawJSON(value, ''))
  if (!text) return '未配置'
  const rows = text.split('\n')
  return rows.slice(0, 2).join('；') + (rows.length > 2 ? ` 等${rows.length}项` : '')
}

function bindStructuredEditors() {
  document.querySelectorAll('.structured-field').forEach(container => {
    const plain = container.querySelector('.structured-input')
    const raw = container.querySelector('.raw-json-editor')
    plain?.addEventListener('input', () => { if (raw) raw.dataset.dirty = 'false' })
    raw?.addEventListener('input', () => { raw.dataset.dirty = 'true' })
  })
}

function dateTimeLocal(value) {
  if (!value) return ''
  const date = new Date(value); if (Number.isNaN(date.getTime())) return ''
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>'"]/g, char => ({ '&':'&amp;', '<':'&lt;', '>':'&gt;', "'":'&#39;', '"':'&quot;' }[char]))
}

function showToast(message, isError = false) {
  const toast = $('#toast'); toast.textContent = message; toast.classList.toggle('error', isError); toast.classList.add('show')
  clearTimeout(state.toastTimer); state.toastTimer = setTimeout(() => toast.classList.remove('show'), 3200)
}

$('#editorForm').addEventListener('submit', saveEditor)
$('#closeEditorButton').addEventListener('click', () => $('#editorDialog').close())
$('#cancelEditorButton').addEventListener('click', () => $('#editorDialog').close())
$('#addButton').addEventListener('click', () => openEditor())
$('#searchInput').addEventListener('input', () => { clearTimeout(state.searchTimer); state.searchTimer = setTimeout(() => { state.page = 1; loadRows() }, 250) })
$('#noticeTypeFilter').addEventListener('change', () => { state.page = 1; loadRows() })
$('#noticePublishedFilter').addEventListener('change', () => { state.page = 1; loadRows() })
$('#pageSizeSelect').addEventListener('change', event => { state.size = Number(event.target.value) || 20; state.page = 1; loadRows() })
$('#prevButton').addEventListener('click', () => { if (state.page > 1) { state.page--; loadRows() } })
$('#nextButton').addEventListener('click', () => { if (state.page * state.size < state.total) { state.page++; loadRows() } })
$('#pageJumpButton').addEventListener('click', jumpToPage)
$('#pageJumpInput').addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); jumpToPage() } })
$('#closeNoticeViewerButton').addEventListener('click', () => $('#noticeViewerDialog').close())
$('#noticeViewerDoneButton').addEventListener('click', () => $('#noticeViewerDialog').close())
$('#exportButton').addEventListener('click', () => { if (state.pageKey === 'menus') return location.href = '/api/menus/export'; if (isConfigPage(state.pageKey) || isReadOnlyPage(state.pageKey) || state.pageKey === 'players' || state.pageKey === 'couples' || state.pageKey === 'realms') return showToast('该页面不支持导出', true); location.href = `/api/export/${resourcePath(state.pageKey)}` })
$('#importButton').addEventListener('click', () => $('#importInput').click())
$('#importInput').addEventListener('change', event => { if (event.target.files[0]) importJSON(event.target.files[0]); event.target.value = '' })
$('#excelImportButton').addEventListener('click', () => state.pageKey === 'menus' ? showToast('菜单请使用 JSON 导入', true) : $('#excelInput').click())
$('#excelInput').addEventListener('change', event => { if (event.target.files[0]) importExcel(event.target.files[0]); event.target.value = '' })
$('#excelExportButton').addEventListener('click', () => { if (state.pageKey === 'menus' || isConfigPage(state.pageKey) || isReadOnlyPage(state.pageKey) || state.pageKey === 'players' || state.pageKey === 'couples' || state.pageKey === 'realms') return showToast('该页面不支持 Excel 导出', true); location.href = `/api/export/${resourcePath(state.pageKey)}?format=xlsx` })
$('#imageButton').addEventListener('click', () => $('#imageInput').click())
$('#imageInput').addEventListener('change', event => { if (event.target.files[0]) uploadImage(event.target.files[0]); event.target.value = '' })
$('#menuButton').addEventListener('click', () => $('#sidebar').classList.toggle('open'))
document.querySelectorAll('[data-jump]').forEach(button => button.addEventListener('click', () => switchPage(button.dataset.jump)))

function jumpToPage() {
  const pagesCount = Math.max(1, Math.ceil(state.total / state.size))
  const requested = Number.parseInt($('#pageJumpInput').value, 10)
  if (!Number.isFinite(requested)) return showToast('请输入有效页码', true)
  const nextPage = Math.min(pagesCount, Math.max(1, requested))
  $('#pageJumpInput').value = String(nextPage)
  if (nextPage === state.page) return
  state.page = nextPage
  loadRows()
}

async function start() {
  await loadNavigation()
  const first = pages.find(page => schemas[page.key]) || fallbackPages[0]
  await switchPage(first.key)
}

start()

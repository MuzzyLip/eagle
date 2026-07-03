// 这里 /pkg/storage/orm 是创建管理数据库实例的包
// 主要功能是创建数据库实例，并管理数据库实例
// 存在冗余设计，设计了一个Manager管理instances map[string]*gorm.DB，但是又设计了一个全局变量DBMap map[string]*gorm.DB来存储数据库实例
// AI建议正式环境只保留其中一个

package orm

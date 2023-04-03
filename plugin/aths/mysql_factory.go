// 定义一个工具包，用来管理gorm数据库连接池的初始化工作。
package aths

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 定义全局的db对象，我们执行数据库操作主要通过他实现。
var _db *gorm.DB

// 包初始化函数，golang特性，每个包初始化的时候会自动执行init函数，这里用来初始化gorm。
func init() {
	var err error
	// 初始化配置
	_config = &Config{}
	err = _config.InitFromYaml("gorm/config.yaml")
	if err != nil {
		panic(err.Error())
	}
	// 连接MySQL数据库
	//dsn := "root:123456@tcp(127.0.0.1:3306)/robot?charset=utf8mb4&parseTime=True&loc=Local"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", _config.MySQL.User, _config.MySQL.Password, _config.MySQL.Host, _config.MySQL.Port, _config.MySQL.DbName)

	// 声明err变量，下面不能使用:=赋值运算符，否则_db变量会当成局部变量，导致外部无法访问_db变量
	//连接MYSQL, 获得DB类型实例，用于后面的数据库读写操作。
	_db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("连接数据库失败, error=" + err.Error())
	}

	sqlDB, _ := _db.DB()

	//设置数据库连接池参数
	sqlDB.SetMaxOpenConns(100) //设置数据库连接池最大连接数
	sqlDB.SetMaxIdleConns(20)  //连接池最大允许的空闲连接数，如果没有sql任务需要执行的连接数大于20，超过的连接会被连接池关闭。
}

// 获取gorm db对象，其他包需要执行数据库查询的时候，只要通过tools.getDB()获取db对象即可。
// 不用担心协程并发使用同样的db对象会共用同一个连接，db对象在调用他的方法的时候会从数据库连接池中获取新的连接
func GetDB() *gorm.DB {
	return _db
}

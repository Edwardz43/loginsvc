package config

import (
	"fmt"

	"github.com/spf13/viper"
)

func init() {
	viper.SetConfigType("json")
	viper.AddConfigPath("./")
	viper.AddConfigPath("../")
	viper.AddConfigPath("../../")
	viper.SetConfigName("config")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	if viper.GetBool(`debug`) {
		fmt.Println("Service RUN on DEBUG mode")
	}
}

func GetSqliteConnectionString() string {
	return viper.GetString("sqliteConnStr")
}

func GetMysqliteConnectionString() string {
	return viper.GetString("mysqlConnStr")
}

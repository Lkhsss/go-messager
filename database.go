package main

import (
	"fmt"

	"gorm.io/gorm"
)

type Data struct {
	DB *gorm.DB
}

func (d *Data) GetPassword(name string) (string, error) {
	var user User
	// 用 Take 而不是 Find：查不到时会返回 gorm.ErrRecordNotFound，便于区分「用户不存在」和「数据库故障」
	if err := d.DB.Where("name = ?", name).Take(&user).Error; err != nil {
		return "", fmt.Errorf("无法查询密码[%w]", err)
	}

	return user.Password, nil
}

func (d *Data) IfUserExist(name string) (bool, error) {
	var count int64
	err := d.DB.Model(&User{}).Where("name = ?", name).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("查询用户名[%w]错误", err)
	}

	exists := count > 0
	return exists, nil
}

func (d *Data) CreateUser(name string, hash string) error {
	user := User{Name: name, Password: hash}

	if err := d.DB.Create(&user).Error; err != nil {
		return fmt.Errorf("创建用户[%s]出错：%w", name, err)
	}
	return nil
}

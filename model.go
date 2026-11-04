package main

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name     string
	Password string
}

type Post struct {
	gorm.Model
	User     string
	Title    string
	Subtitle string
	Body     string
}

// 设置的条目
type Setting struct {
	gorm.Model
	Key   string
	Value string
}

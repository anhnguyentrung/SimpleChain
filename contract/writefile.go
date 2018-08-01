package main

import (
	"io/ioutil"
	"encoding/json"
	"fmt"
)

type User struct {
	Id int
	Name string
}

func NewUser() User{
	return User{Id: 1, Name:"kenny"}
}

func main() {
	user := NewUser()
	userJson, err := json.Marshal(user)
	if err != nil {
		fmt.Println(err)
	}
	ioutil.WriteFile("user.json", userJson, 0644)
	fmt.Println(userJson)
}

package main

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func main() {
	x := primitive.NewObjectID()
	fmt.Println(x)

}

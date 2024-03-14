package main

import (
	"errors"
	"fmt"
	"time"
)

func main() {
	fmt.Printf("int: %s\n", 10)
	hailGod()
	tringo()
	//s := "bingo"
	//t := &s
	//pointer.StringDeref(t)
	//t = nil
	//pointer.StringDeref(t)
}

func APIMetricRecorderFn(azServiceName string, errPtr *error) func() {
	invocationTime := time.Now()
	return func() {
		RecordAzAPIMetric(*errPtr, azServiceName, invocationTime)
	}
}

func RecordAzAPIMetric(err error, operation string, invocationTime time.Time) {
	if err != nil {
		fmt.Printf("RecordAzAPIMetric: err: %s, operation:%s, invocationTime: %s\n", err, operation, invocationTime)
	} else {
		fmt.Printf("RecordAzAPIMetric: err: nils, operation:%s, invocationTime: %s\n", operation, invocationTime)
	}
}

func hailGod() error {
	var err error
	defer APIMetricRecorderFn("hailgod", &err)()
	fmt.Println("God Service Called")
	err = errors.New("noreply")
	return err
}

func tringo() (err error) {
	defer APIMetricRecorderFn("tringo", &err)()
	fmt.Println("tringocalled")
	return err
}

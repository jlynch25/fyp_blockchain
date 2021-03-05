package ticketing

import "time"

type event struct {
	Timestamp       int64
	// Hash            []bte
	totalNumTickets int
	title           string
	location        location
	dateTime        dateTime
	guests          []guest
}

type location struct {
	name        string
	coordinates coordinates
}

type coordinates struct {
	latitude  float32
	longitude float32
}

type date struct {
	year  int
	month time.Month // Month is 1 - 12. 0 means unspecified
	day   int
}

type dateTime struct {
	startTime time.Time
	endTime   time.Time
	date      date
}

type guest struct {
	name          string
	Timestamp     string
	claranceLevel []string // normal, vip, backstage ...
	seatLocation  string   //TODO look into this (seat alication)
	// transaction   *Transacton
}

// this is for in transation data
//
// say we ahve a blockchain per event
// 	block per ticket type ??
//
//
type data struct {
	seatlocation string
}

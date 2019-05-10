// 	"strconv"

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

// Config struct defines overall endpoint  setup
type Config struct {
	Simulator bool       `json:"simulator"` // True = run simulator
	Cat       []Category `json:"categories"`
	Sens      []Sensor   `json:"sensors"`
	Success   float64    `json:"successRate"` // success rate (fraction of one value outputs for simulator)
	ErrRate   float64    `json:"errorRate"`   // error rate for category (switch threshold to opposite)
}

// Category struct defines category parameters
type Category struct {
	Label         string    `json:"name"`           // name of sensor parameter
	Level         []string  `json:"levels"`         // all levels are defined as strings (even numbers)
	ZeroThreshold []float64 `json:"zeroThresholds"` // thresholds for uniform [0,1] rv simulator zero output
	OneThreshold  []float64 `json:"oneThresholds"`  // thresholds for uniform [0,1] rv simulator one output
}

// Sensor struct defines a sensor
// first four parameters define sensor; remaining parameters are for simulator
// sensor simulators are as follows:
//  	First sublist entry (case)
//            0 = two means correlated with two class (normal distributions)
//            1 = two means anti-correlated with  two class (normal distributions)
//            2 = one mean -- no correlation (normal distribution) - use only zero mean and std dev
//            3 = uniformly distributed [0, 1] with no correlation
//      The scale factor multiplies the rv for the output
//		Two output classes are assumed (0 and 1 or pass and fail)
type Sensor struct {
	Label        string  `json:"name"`          // name of sensor parameter
	UpperLimit   float64 `json:"upperLimit"`    // max value for sensor
	LowerLimit   float64 `json:"lowerLimit"`    // min value for sensor
	UpperControl float64 `json:"upperControl"`  // upper warning or control for sensor
	LowerControl float64 `json:"lowerControl"`  // lower warning or control for sensor
	SimType      int     `json:"simulatorType"` // simulator type
	ZeroMean     float64 `json:"zeroMean"`      // mean of zero output simulator
	ZeroStdDev   float64 `json:"zeroStdDev"`    // std dev of zero output simulator
	OneMean      float64 `json:"oneMean"`       // mean of one output simulator
	OneStdDev    float64 `json:"oneStdDev"`     // std dev of one output simulator
	Scale        float64 `json:"scale"`         // scale of simulator
}

// EpdData struct has simulator or endpoint outputs for categories and sensors
type EpdData struct {
	CurrentTime string             `json:"time"`
	Topic       string             `json:"endpoint"`
	Cat         map[string]string  `json:"categories"`
	Sens        map[string]float64 `json:"sensors"`
	Result      int                `json:"result"`
}

func readConfigData(epdConf string) Config {

	// Open json file
	jsonFile, err := os.Open(epdConf)
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully Opened epd.conf file")

	// defer the closing of our json file so it can be parsed
	defer jsonFile.Close()

	// read json file as a byte array.
	b, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully read file as byteArray")

	// unmarshal byteArray which contains json content into 'config'
	var config Config
	err = json.Unmarshal(b, &config)
	if err != nil {
		debug.PrintStack()
		panic(err)
	}

	// print config struct
	fmt.Printf("%+v", config)
	fmt.Println(" ")
	fmt.Println(" ")

	return config
}

func makeSimulatedRecord(config Config, topic string) string {
	// initialize epdData struct
	epd := &EpdData{
		Cat:  make(map[string]string),
		Sens: make(map[string]float64),
	}

	epd.Topic = topic
	// get current time and convert to formatted string
	epd.CurrentTime = time.Now().Format("2006-01-02 15:04:05.000")

	// determine output (1 or 0) for data record
	if rand.Float64() < config.Success {
		epd.Result = 1
	} else {
		epd.Result = 0
	}

	// generate sensor endpoint (numerics) simulated data
	for i := range config.Sens {
		s := config.Sens[i]
		// use switch to determine simulator type
		output := 0.0
		switch s.SimType {
		case 0:
			if epd.Result == 1 {
				output = rand.NormFloat64()*s.OneStdDev + s.OneMean
			} else {
				output = rand.NormFloat64()*s.ZeroStdDev + s.ZeroMean
			}
		case 1:
			if epd.Result == 1 {
				output = rand.NormFloat64()*s.ZeroStdDev + s.ZeroMean
			} else {
				output = rand.NormFloat64()*s.OneStdDev + s.OneMean
			}
		case 2:
			output = rand.NormFloat64()*s.OneStdDev + s.OneMean
		case 3:
			output = rand.Float64()
		default:
			fmt.Printf("Improper simulator type for record: %v \n", i)
		}
		epd.Sens[s.Label] = math.Round(1000*s.Scale*output) / 1000
	}

	// generate category endpoint simulated data
	for i := range config.Cat {
		c := config.Cat[i]
		epd.Cat[c.Label] = "None"
		// generate category level from uniform rv
		if epd.Result == 1 {
			for j := range c.OneThreshold {
				if rand.Float64() < c.OneThreshold[j] {
					epd.Cat[c.Label] = c.Level[j]
					break
				}
			}
		} else {
			for j := range c.ZeroThreshold {
				if rand.Float64() < c.ZeroThreshold[j] {
					epd.Cat[c.Label] = c.Level[j]
					break
				}
			}
		}
		// add variability to simulated data with category error rate
		if rand.Float64() < config.ErrRate {
			index := rand.Int31n(int32(len(c.Level)))
			epd.Cat[c.Label] = c.Level[index]
		}
	}

	dataRecord := fmt.Sprintf(`{"result":%v,"time":%s,`, epd.Result, epd.CurrentTime)

	jsonString, err := json.Marshal(epd.Sens)
	if err != nil {
		panic(err)
	}
	s := strings.Trim(string(jsonString), "{")
	s = strings.Trim(s, "}")
	dataRecord += string(s) + ","

	jsonString, err = json.Marshal(epd.Cat)
	if err != nil {
		panic(err)
	}

	s = strings.Trim(string(jsonString), "{")
	dataRecord += string(s)
	dataRecord = strings.Replace(dataRecord, ",", ", ", -1)
	dataRecord = strings.Replace(dataRecord, ":", " : ", -1)
	return dataRecord
}

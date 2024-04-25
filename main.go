package main

import (
	"flag"
	"log"

	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/postgres"
	"github.com/blocto/solana-go-sdk/rpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var config Config

// Cluster enum
type Cluster string

var database *gorm.DB

const useGCloudDB = true

type ClustersToRun string

// Cluster enum
const (
	MainnetBeta Cluster = "MainnetBeta"
	Testnet             = "Testnet"
	Devnet              = "Devnet"
)

var influxdb *InfluxdbClient
var userInputClusterMode string
var mainnetFailover RPCFailover
var testnetFailover RPCFailover
var devnetFailover RPCFailover

const (
	RunMainnetBeta ClustersToRun = "mainnet"
	RunTestnet                   = "testnet"
	RunDevnet                    = "devnet"
	RunAllClusters               = "all"
)

func init() {
	config = loadConfig()
	log.Println(" *** Config Start *** ")
	log.Println("--- //// Database Config --- ")
	log.Println(config.Database)
	log.Println("--- //// Influxdb Config --- ")
	log.Println(config.InfluxdbConfig)
	log.Println("--- //// Retension --- ")
	log.Println(config.Retension)
	log.Println("--- //// ClusterCLIConfig--- ")
	log.Println("ClusterCLIConfig Mainnet", config.ClusterCLIConfig.ConfigMain)
	log.Println("ClusterCLIConfig Testnet", config.ClusterCLIConfig.ConfigTestnet)
	log.Println("ClusterCLIConfig Devnet", config.ClusterCLIConfig.ConfigDevnet)
	log.Println("--- Mainnet Ping  --- ")
	log.Println("Mainnet.ClusterPing.APIServer", config.Mainnet.ClusterPing.APIServer)
	log.Println("Mainnet.ClusterPing.PingServiceEnabled", config.Mainnet.ClusterPing.PingServiceEnabled)
	log.Println("Mainnet.ClusterPing.AlternativeEnpoint.HostList", config.Mainnet.ClusterPing.AlternativeEnpoint.HostList)
	log.Println("Mainnet.ClusterPing.PingConfig", config.Mainnet.ClusterPing.PingConfig)
	log.Println("Mainnet.ClusterPing.Report", config.Mainnet.ClusterPing.Report)
	log.Println("--- Testnet Ping  --- ")
	log.Println("Mainnet.ClusterPing.APIServer", config.Testnet.ClusterPing.APIServer)
	log.Println("Mainnet.ClusterPing.PingServiceEnabled", config.Mainnet.ClusterPing.PingServiceEnabled)
	log.Println("Testnet.ClusterPing.AlternativeEnpoint.HostList", config.Testnet.ClusterPing.AlternativeEnpoint.HostList)
	log.Println("Testnet.ClusterPing.PingConfig", config.Testnet.ClusterPing.PingConfig)
	log.Println("Testnet.ClusterPing.Report", config.Testnet.ClusterPing.Report)
	log.Println("--- Devnet Ping  --- ")
	log.Println("Devnet.ClusterPing.APIServer", config.Devnet.ClusterPing.APIServer)
	log.Println("Devnet.ClusterPing.Enabled", config.Devnet.ClusterPing.PingServiceEnabled)
	log.Println("Devnet.ClusterPing.AlternativeEnpoint.HostList", config.Devnet.ClusterPing.AlternativeEnpoint.HostList)
	log.Println("Devnet.ClusterPing.PingConfig", config.Devnet.ClusterPing.PingConfig)
	log.Println("Devnet.ClusterPing.Report", config.Devnet.ClusterPing.Report)

	log.Println(" *** Config End *** ")

	ResponseErrIdentifierInit()
	StatisticErrExpectionInit()
	AlertErrExpectionInit()
	ReportErrExpectionInit()
	PingTakeTimeErrExpectionInit()

	if config.DBConn == "" {
		log.Println("dbconn is empty. won't init database")
	} else {
		if config.Database.UseGoogleCloud {
			gormDB, err := gorm.Open(postgres.New(postgres.Config{
				DriverName: "cloudsqlpostgres",
				DSN:        config.DBConn,
			}))
			if err != nil {
				log.Panic(err)
			}
			database = gormDB
		} else {
			gormDB, err := gorm.Open(postgres.Open(config.DBConn), &gorm.Config{})
			if err != nil {
				log.Panic(err)
			}
			database = gormDB
		}
		log.Println("database connected")
	}

	if config.InfluxdbConfig.Enabled {
		influxdb = NewInfluxdbClient(config.InfluxdbConfig)
	}
	/// ---- Start RPC Failover ---
	log.Println("RPC Endpoint Failover Setting ---")
	if len(config.Mainnet.AlternativeEnpoint.HostList) <= 0 {
		mainnetFailover = NewRPCFailover([]RPCEndpoint{{
			Endpoint: rpc.MainnetRPCEndpoint,
			Piority:  1,
			MaxRetry: 30}})
	} else {
		mainnetFailover = NewRPCFailover(config.Mainnet.AlternativeEnpoint.HostList)
	}
	if len(config.Testnet.AlternativeEnpoint.HostList) <= 0 {
		testnetFailover = NewRPCFailover([]RPCEndpoint{{
			Endpoint: rpc.MainnetRPCEndpoint,
			Piority:  1,
			MaxRetry: 30}})
	} else {
		testnetFailover = NewRPCFailover(config.Testnet.AlternativeEnpoint.HostList)
	}
	if len(config.Devnet.AlternativeEnpoint.HostList) <= 0 {
		devnetFailover = NewRPCFailover([]RPCEndpoint{{
			Endpoint: rpc.MainnetRPCEndpoint,
			Piority:  1,
			MaxRetry: 30}})
	} else {
		devnetFailover = NewRPCFailover(config.Devnet.AlternativeEnpoint.HostList)
	}
}

func main() {
	defer func() {
		if influxdb != nil {
			influxdb.ClientClose()
		}
		if database != nil {
			sqldb, err := database.DB()
			if err == nil {
				sqldb.Close()
			}
		}
	}()
	flag.Parse()
	clustersToRun := ClustersToRun(flag.Arg(0))

	switch clustersToRun {
	case RunMainnetBeta, RunTestnet, RunDevnet, RunAllClusters:
		log.Printf("run for %v\n", clustersToRun)

		log.Printf("start workers")
		go launchWorkers(ClustersToRun(clustersToRun))

		log.Println("start api service")
		go APIService(ClustersToRun(clustersToRun))
	default:
		log.Fatalf("unexpected arg: %v\n", clustersToRun)
	}

	select {}
}

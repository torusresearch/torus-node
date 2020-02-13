package config

/* All useful imports */
import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sync"

	"github.com/caarlos0/env"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
)

var (
	logLevelMap = map[string]logging.Level{
		// PanicLevel level, highest level of severity. Logs and then calls panic with the
		// message passed to Debug, Info, ...
		"panic": logging.PanicLevel,
		// FatalLevel level. Logs and then calls `logger.Exit(1)`. It will exit even if the
		// logging level is set to Panic.
		"fatal": logging.FatalLevel,
		// ErrorLevel level. Logs. Used for errors that should definitely be noted.
		// Commonly used for hooks to send errors to an error tracking service.
		"error": logging.ErrorLevel,
		// WarnLevel level. Non-critical entries that deserve eyes.
		"warn": logging.WarnLevel,
		// InfoLevel level. General operational entries about what's going on inside the
		// application.
		"info": logging.InfoLevel,
		// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
		"debug": logging.DebugLevel,
		// TraceLevel level. Designates finer-grained informational events than the Debug.
		"trace": logging.TraceLevel,
	}
)

var GlobalConfig *Config
var GlobalMutableConfig *MutableConfig

type Config struct {
	HttpServerPort string `json:"httpServerPort" env:"HTTP_SERVER_PORT"`

	// Used to manage the node whilst in live
	ManagementRPCPort string `json:"managementRPCPort" env:"MANAGEMENT_RPC_PORT"`
	UseManagementRPC  bool   `json:"useManagementRPC" env:"USE_MANAGEMENT_RPC"`

	// NOTE: This is what is used for registering on the Ethereum network.
	MainServerAddress          string `json:"mainServerAddress" env:"MAIN_SERVER_ADDRESS"`
	EthConnection              string `json:"ethconnection" env:"ETH_CONNECTION"`
	EthPrivateKey              string `json:"ethprivatekey" env:"ETH_PRIVATE_KEY"`
	BftURI                     string `json:"bfturi" env:"BFT_URI"`
	ABCIServer                 string `json:"abciserver" env:"ABCI_SERVER"`
	TMP2PListenAddress         string `json:"tmp2plistenaddress" env:"TM_P2P_LISTEN_ADDRESS"`
	P2PListenAddress           string `json:"p2plistenaddress" env:"P2P_LISTEN_ADDRESS"`
	NodeListAddress            string `json:"nodelistaddress" env:"NODE_LIST_ADDRESS"`
	KeyBuffer                  int    `json:"keybuffer" env:"KEY_BUFFER"`
	KeysPerEpoch               int    `json:"keysperepoch" env:"KEYS_PER_EPOCH"`
	KeyBufferTriggerPercentage int    `json:"keybuffertriggerpercentage" env:"KEY_BUFFER_TRIGGER_PERCENTAGE"`

	// KeyBufferTriggerPercentage int    `json:"keybuffertriggerpercetage" env:"KEY_BUFFER_TRIGGER_PERCENTAGE"` // percetage threshold of keys left to trigger buffering 90 - 20
	BasePath  string `json:"basepath" env:"BASE_PATH"`
	InitEpoch int    `json:"initepoch" env:"INIT_EPOCH"`

	ShouldRegister          bool   `json:"register" env:"REGISTER"`
	CPUProfileToFile        string `json:"cpuProfile" env:"CPU_PROFILE"`
	IsDebug                 bool   `json:"debug" env:"DEBUG"`
	PprofPort               string `json:"pprofPort" env:"PPROF_PORT"`
	PprofEnabled            bool   `json:"pprofEnabled" env:"PPROF_ENABLED"`
	ProvidedIPAddress       string `json:"ipAddress" env:"IP_ADDRESS"`
	LogLevel                string `json:"loglevel" env:"LOG_LEVEL"`
	TendermintLogging       bool   `json:"tendermintLogging" env:"TENDERMINT_LOGGING"`
	TMMessageQueueProcesses int    `json:"tmMessageQueueProcesses" env:"TM_MESSAGE_QUEUE_PROCESSES"`
	MaxBftConnections       int    `json:"maxBftConnections" env:"MAX_BFT_CONNECTIONS"`
	StackImpactEnabled      bool   `json:"stackImpactEnabled" env:"STACK_IMPACT_ENABLED"`
	StackImpactKey          string `json:"stackImpactKey" env:"STACK_IMPACT_KEY"`

	ServeUsingTLS     bool   `json:"useTLS" env:"USE_TLS"`
	UseAutoCert       bool   `json:"useAutoCert" env:"USE_AUTO_CERT"`
	AutoCertCacheDir  string `json:"autoCertCacheDir" env:"AUTO_CERT_CACHE_DIR"`
	PublicURL         string `json:"publicURL" env:"PUBLIC_URL"`
	ServerCert        string `json:"serverCert" env:"SERVER_CERT"`
	ServerKey         string `json:"serverKey" env:"SERVER_KEY"`
	EthPollFreq       int    `json:"ethPollFreq" env:"ETH_POLL_FREQ"`
	TendermintMetrics bool   `json:"tendermintMetrics" env:"TENDERMINT_METRICS"`

	// IDs used for oauth verification.
	GoogleClientID       string `json:"googleClientID" env:"GOOGLE_CLIENT_ID" mutable:"yes"`
	FacebookAppID        string `json:"facebookAppID" env:"FACEBOOK_APP_ID" mutable:"yes"`
	FacebookAppSecret    string `json:"facebookAppSecret" env:"FACEBOOK_APP_SECRET" mutable:"yes"`
	TwitchClientID       string `json:"twitchClientID" env:"TWITCH_CLIENT_ID" mutable:"yes"`
	RedditClientID       string `json:"redditClientID" env:"REDDIT_CLIENT_ID" mutable:"yes"`
	DiscordClientID      string `json:"discordClientID" env:"DISCORD_CLIENT_ID" mutable:"yes"`
	TorusVerifierPubKeyX string `json:"torusVerifierPubKeyX" env:"TORUS_VERIFIER_PUB_KEY_X" mutable:"yes"`
	TorusVerifierPubKeyY string `json:"torusVerifierPubKeyY" env:"TORUS_VERIFIER_PUB_KEY_Y" mutable:"yes"`
	EncryptShares        bool   `json:"encryptShares" env:"ENCRYPT_SHARES" mutable:"yes"`
	ValidateDealer       bool   `json:"validateDealer" env:"VALIDATE_DEALER" mutable:"yes"`

	JRPCAuth        bool   `json:"jrpcAuth" env:"JRPC_AUTH" mutable:"yes"`
	JRPCAuthPubKeyX string `json:"jrpcAuthPubKeyX" env:"JRPC_AUTH_PUB_KEY_X" mutable:"yes"`
	JRPCAuthPubKeyY string `json:"jrpcAuthPubKeyY" env:"JRPC_AUTH_PUB_KEY_Y" mutable:"yes"`

	PSSShareDelayMS         int  `json:"pssShareDelayMS" env:"PSS_SHARE_DELAY_MS" mutable:"yes"`
	DiskQueueReadRateMS     int  `json:"diskQueueReadRateMS" env:"DISK_QUEUE_READ_RATE_MS" mutable:"yes"`
	DiskQueueHWMMB          int  `json:"diskQueueHWMMB" env:"DISK_QUEUE_HWM_MB" mutable:"yes"`
	DiskQueueWriteToDB      bool `json:"diskQueueWriteToDB" env:"DISK_QUEUE_WRITE_TO_DB" mutable:"yes"`
	DiskQueueLogLevel       int  `json:"diskQueueLogLeve" env:"DISK_QUEUE_LOG_LEVEL" mutable:"yes"`
	StaggerDelay            int  `json:"staggerDelay" env:"STAGGER_DELAY"`
	OSFreeMemoryMS          int  `json:"osFreeMemoryMS" env:"OS_FREE_MEMORY_MS" mutable:"yes"`
	IgnoreEpochForKeyAssign bool `json:"ignoreEpochForKeyAssign" env:"IGNORE_EPOCH_FOR_KEY_ASSIGN" mutable:"yes"`
}

type MutableConfig struct {
	sync.Map
}

func (m *MutableConfig) Exists(key string) bool {
	_, exists := m.Load(key)
	return exists
}
func (m *MutableConfig) GetS(key string) string {
	inter, _ := m.Load(key)
	val, ok := inter.(string)
	if !ok {
		logging.WithError(fmt.Errorf("Could not cast result %v from MutableConfig to string ", val)).Error()
	}
	return val
}
func (m *MutableConfig) SetS(key string, value string) {
	m.Store(key, value)
	logging.Debugf("Mutable Configs: %s", m.ListMutableConfigs())
}
func (m *MutableConfig) GetI(key string) int {
	inter, _ := m.Load(key)
	val, ok := inter.(int)
	if !ok {
		logging.WithError(fmt.Errorf("Could not cast result %v from MutableConfig to int ", val)).Error()
	}
	return val
}
func (m *MutableConfig) SetI(key string, value int) {
	m.Store(key, value)
	logging.Debugf("Mutable Configs: %s", m.ListMutableConfigs())
}
func (m *MutableConfig) GetB(key string) bool {
	inter, _ := m.Load(key)
	val, ok := inter.(bool)
	if !ok {
		logging.WithError(fmt.Errorf("Could not cast result %v from MutableConfig to bool ", val)).Error()
	}
	return val
}
func (m *MutableConfig) SetB(key string, value bool) {
	m.Store(key, value)
	logging.Debugf("Mutable Configs: %s", m.ListMutableConfigs())
}
func (m *MutableConfig) ListMutableConfigs() string {
	con := make(map[string]interface{})
	m.Range(func(k, v interface{}) bool {
		keyStr, ok := k.(string)
		if ok {
			con[keyStr] = v
		} else {
			logging.Errorf("ListMutableConfigs has a non-string key %v", k)
		}
		return true
	})
	str, err := bijson.Marshal(con)
	if err != nil {
		logging.WithError(err).Error("could not marshal ListMutableConfigs")
		return ""
	}
	return string(str)
}

func InitMutableConfig(c *Config) *MutableConfig {
	m := &MutableConfig{}
	t := reflect.TypeOf(c).Elem()
	config := reflect.Indirect(reflect.ValueOf(c))
	for i := 0; i < config.NumField(); i++ {
		fieldValue := config.Field(i)
		field := t.Field(i)
		tag := field.Tag.Get("mutable")
		fieldType := field.Type.String()
		if tag == "yes" {
			if fieldType == "string" {
				m.SetS(field.Name, fieldValue.String())
			} else if fieldType == "int" {
				m.SetI(field.Name, int(fieldValue.Int()))
			} else if fieldType == "bool" {
				m.SetB(field.Name, fieldValue.Bool())
			}
		}
	}
	return m
}

// mergeWithFlags explicitly merges flags for a given instance of Config
// NOTE: It will note override with defaults
func (c *Config) mergeWithFlags(flagConfig *Config) *Config {

	if isFlagPassed("register") {
		c.ShouldRegister = flagConfig.ShouldRegister
	}
	if isFlagPassed("debug") {
		c.IsDebug = flagConfig.IsDebug
	}
	if isFlagPassed("ethprivateKey") {
		c.EthPrivateKey = flagConfig.EthPrivateKey
	}
	if isFlagPassed("ipAddress") {
		c.ProvidedIPAddress = flagConfig.ProvidedIPAddress
	}
	if isFlagPassed("cpuProfile") {
		c.CPUProfileToFile = flagConfig.CPUProfileToFile
	}
	if isFlagPassed("ethConnection") {
		c.EthConnection = flagConfig.EthConnection
	}
	if isFlagPassed("nodeListAddress") {
		c.NodeListAddress = flagConfig.NodeListAddress
	}
	if isFlagPassed("basePath") {
		c.BasePath = flagConfig.BasePath
	}

	return c
}

// createConfigWithFlags edits a config with flags parsed in.
// NOTE: It will note override with defaults
func (c *Config) createConfigWithFlags() string {
	register := flag.Bool("register", true, "defaults to true")
	debug := flag.Bool("debug", false, "defaults to false")
	ethPrivateKey := flag.String("ethprivateKey", "", "provide private key here to run node on")
	ipAddress := flag.String("ipAddress", "", "specified IPAdress, necessary for running in an internal env e.g. docker")
	cpuProfile := flag.String("cpuProfile", "", "write cpu profile to file")
	ethConnection := flag.String("ethConnection", "", "ethereum endpoint")
	nodeListAddress := flag.String("nodeListAddress", "", "node list address on ethereum")
	basePath := flag.String("basePath", "/.torus", "basePath for Torus node artifacts")
	configPath := flag.String("configPath", "", "override configPath")
	flag.Parse()

	if isFlagPassed("register") {
		c.ShouldRegister = *register
	}
	if isFlagPassed("debug") {
		c.IsDebug = *debug
	}
	if isFlagPassed("ethprivateKey") {
		c.EthPrivateKey = *ethPrivateKey
	}
	if isFlagPassed("ipAddress") {
		c.ProvidedIPAddress = *ipAddress
	}
	if isFlagPassed("cpuProfile") {
		c.CPUProfileToFile = *cpuProfile
	}
	if isFlagPassed("ethConnection") {
		c.EthConnection = *ethConnection
	}
	if isFlagPassed("nodeListAddress") {
		c.NodeListAddress = *nodeListAddress
	}
	if isFlagPassed("basePath") {
		c.BasePath = *basePath
	}

	return *configPath
}

// Source: https://stackoverflow.com/a/54747682
func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func readAndMarshallJSONConfig(configPath string, c *Config) error {
	jsonConfig, err := os.Open(configPath)
	if err != nil {
		return err
	}

	defer jsonConfig.Close()

	b, err := ioutil.ReadAll(jsonConfig)
	if err != nil {
		return err
	}

	err = bijson.Unmarshal(b, &c)
	if err != nil {
		return err
	}

	return nil
}

func LoadConfig(configPath string) *Config {

	// Default config is initalized here
	conf := DefaultConfigSettings()
	flagConf := DefaultConfigSettings()
	providedCF := flagConf.createConfigWithFlags()
	if providedCF != "" {
		logging.WithField("configPath", providedCF).Info("overriding configPath")
		configPath = providedCF
	}

	err := readAndMarshallJSONConfig(configPath, &conf)
	if err != nil {
		logging.WithError(err).Warning("failed to read JSON config")
	}

	err = env.Parse(&conf)
	if err != nil {
		logging.WithError(err).Error("could not parse config")
	}

	conf.mergeWithFlags(&flagConf)

	logging.SetLevel(logLevelMap[conf.LogLevel])

	// TEAM: If you want to use localhost just explicitly pass it as an env / flag...
	// if !conf.IsDebug {
	// 	conf.MainServerAddress = "localhost" + ":" + conf.HttpServerPort
	// }
	// retrieve map[string]interface{}

	if conf.ProvidedIPAddress != "" {
		logging.WithField("IPAddress", conf.ProvidedIPAddress).Info("Running")
		conf.MainServerAddress = conf.ProvidedIPAddress + ":" + conf.HttpServerPort
		conf.P2PListenAddress = fmt.Sprintf("/ip4/%s/tcp/1080", conf.ProvidedIPAddress)
	}
	conf.MainServerAddress = "0.0.0.0" + ":" + conf.HttpServerPort
	conf.P2PListenAddress = fmt.Sprintf("/ip4/%s/tcp/1080", "0.0.0.0")

	bytConf, _ := bijson.Marshal(conf)
	logging.WithField("finalConfiguration", string(bytConf)).Info()

	return &conf
}

func DefaultConfigSettings() Config {
	return Config{}
}

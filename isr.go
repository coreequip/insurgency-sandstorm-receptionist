package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	connectTimeout = 3
	version        = "0.2"
)

type ServerInfo struct {
	name       string
	srvmap     string
	players    int
	maxPlayers int
	bots       int
}

var (
	configFile    = "./isr.cfg"
	messageIdFile = "./message.id"
	playerList    = map[string]bool{}

	hasRules = false
	useBot   = false
	ruleSet  []string

	conf = struct {
		Host             string `name:"host"`
		QueryPort        int    `name:"queryport"`
		RconPort         int    `name:"rconport"`
		RconPassword     string `name:"rconpassword"`
		TemplateWelcome  string `name:"templatewelcome"`
		TemplateFarewell string `name:"templatefarewell"`
		RuleFile         string `name:"rulefile"`
		BotToken         string `name:"bottoken"`
		ChannelId        string `name:"channelid"`

		TellFirstRuleDelay int `name:"tellfirstruledelay"`
		TellNextRulesDelay int `name:"tellnextrulesdelay"`
	}{
		Host:               "",
		QueryPort:          27131,
		RconPort:           27015,
		RconPassword:       "",
		TemplateWelcome:    "Welcome, @!",
		TemplateFarewell:   "Player @ just left.",
		RuleFile:           "rules.txt",
		TellFirstRuleDelay: 30,
		TellNextRulesDelay: 10,
	}
	serverInfo ServerInfo
)

func main() {
	startup()

	playRules := struct {
		doPlay    bool
		nextRule  time.Time
		ruleIndex int
	}{doPlay: false, nextRule: time.Now(), ruleIndex: 0}

	log.Println("Start monitoring")
	for {
		players, err := readPlayers()
		if err != nil {
			log.Println(err.Error())
			time.Sleep(30 * time.Second)
			continue
		}

		// check for joined players
		for name := range players {
			if strings.TrimSpace(name) == "" {
				continue
			}
			_, found := playerList[name]
			if found {
				if hasRules && playerList[name] && players[name].Seconds() > float64(conf.TellFirstRuleDelay) {
					playerList[name] = false
					if !playRules.doPlay {
						playRules.doPlay = true
						playRules.nextRule = nextTime(conf.TellNextRulesDelay)
						playRules.ruleIndex = 0
					}
				}
				continue
			}
			playerList[name] = true

			rconSay(strings.Replace(conf.TemplateWelcome, "@", name, -1))
			updateBotText()
		}

		// check for parted players
		for name := range playerList {
			_, found := players[name]
			if found {
				continue
			}
			delete(playerList, name)

			rconSay(strings.Replace(conf.TemplateFarewell, "@", name, -1))
			updateBotText()
		}

		if playRules.doPlay && time.Now().After(playRules.nextRule) {
			rconSay(ruleSet[playRules.ruleIndex])
			playRules.ruleIndex++
			playRules.nextRule = nextTime(conf.TellNextRulesDelay)

			if playRules.ruleIndex >= len(ruleSet) {
				playRules.doPlay = false
			}
		}

		time.Sleep(5 * time.Second)
	}

}

func loadRules() {
	file, err := os.Open(conf.RuleFile)

	hasRules = err == nil && file != nil
	if hasRules {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			ruleSet = append(ruleSet, line)
		}
		_ = file.Close()
	} else {
		log.Println("WARNING: Can't find rules file. Rules disabled.")
	}
}

type logWriter struct{}

func (writer logWriter) Write(bytes []byte) (int, error) {
	return fmt.Print(time.Now().UTC().Format("[2006-01-02T15:04:05.999Z] ") + string(bytes))
}

func startup() {
	log.SetFlags(0)
	log.SetOutput(new(logWriter))

	printHeader()

	if len(os.Args) > 1 {
		param := strings.ToLower(os.Args[1])
		if param == "--help" || param == "-h" {
			printUsage("", true)
		}

		configFile = os.Args[1]
	}

	configFile, _ = filepath.Abs(configFile)

	_, err := os.Stat(configFile)
	if os.IsNotExist(err) {
		printUsage(fmt.Sprintf("ERROR: Config file \"%s\" doesn't exist!", configFile), false)
	}

	loadConfig()
	loadRules()

	useBot = conf.BotToken != "" && conf.ChannelId != ""

	if useBot {
		BotInit(conf.BotToken, conf.ChannelId)
		updateBotText()
	}
}

func loadConfig() {
	file, err := os.Open(configFile)

	if err != nil || file == nil {
		printUsage("ERROR: Can't open config file.", false)
	}
	defer file.Close()

	t := reflect.TypeOf(conf)
	confMap := map[string]string{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		confMap[field.Tag.Get("name")] = field.Name
	}
	confValues := reflect.ValueOf(&conf).Elem()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		splitLine := strings.Split(line, "=")
		if len(splitLine) < 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(splitLine[0]))
		val := strings.TrimSpace(strings.Join(splitLine[1:], "="))

		fieldName, found := confMap[key]
		if !found {
			continue
		}

		fieldValue := confValues.FieldByName(fieldName)
		if fieldValue.Kind() == reflect.String {
			fieldValue.SetString(val)
		}
		if fieldValue.Kind() == reflect.Int {
			if intVal, err := strconv.Atoi(val); err == nil {
				fieldValue.SetInt(int64(intVal))
			}
		}
	}

	if conf.Host == "" {
		printUsage("ERROR: No host specified in config.", false)
	}
}

func printUsage(message string, full bool) {
	exitCode := 0
	if message != "" {
		message = message + "\n"
		exitCode = 1
	}

	fmt.Printf(` %s
 Usage:
   isr [-h|--help] [configfile]

 ISR tries to load a isr.cfg from the current directory
 unless the "configfile" argument specifies a different
 location.

`, message)
	if full {
		fmt.Println(" Example config file:")
		_, _ = fmt.Fprint(os.Stderr, `
# hostname or ip of your server
host                = my.fancy.server

# server's query port
queryPort           = 27131

# server's RCON port
rconPort            = 27015

# RCON password
rconPassword        = supersecret

# welcome message, @ is replaced with players name
templateWelcome     = Welcome, @!

# farewell message
templateFarewell    = Player @ just left.

# rules file
rulesFile           = rules.txt

# delay until first rule is printed in seconds
tellFirstRuleDelay = 30

# delay between every rule in seconds
tellNextRulesDelay = 10

`)
		_, exe := filepath.Split(os.Args[0])
		fmt.Printf("\n You can use\n\n   %s --help 2>isr.cfg\n\n to copy this example into a file.", exe)
	} else {
		fmt.Println(" Use --help to see a example.")
	}

	os.Exit(exitCode)
}

func printHeader() {
	title := fmt.Sprintf("Insurgency Sandstorm Receptionist v%s", version)
	fmt.Printf(`
  ___  ___________________
 |   |/   _____/\______   \
 |   |\_____  \  |       _/
 |   |/        \ |    |   \
 |___/_______  / |____|_  /
             \/         \/

 %s
 %s
`, title, strings.Repeat("~", len(title)))
}

func updateBotText() {
	if !useBot {
		return
	}

	players := make([]string, 0, len(playerList))
	for k := range playerList {
		players = append(players, k)
	}

	err := BotSendMessage(fmt.Sprintf(
		"Server: <b>%s</b>\n\nPlayers <b>%d</b> / %d:\n\n- <b>%s</b>\n\nBots: %d\nMap: <b>%s</b>\nLast update: %s",
		serverInfo.name,
		serverInfo.players,
		serverInfo.maxPlayers,
		strings.Join(players, "\n- "),
		serverInfo.bots,
		serverInfo.srvmap,
		time.Now().Local().Format("2006-01-02 15:04:05")))

	if err != nil {
		log.Println("ERROR sending message:", err.Error())
	}
}

func readPlayers() (map[string]time.Duration, error) {
	const (
		MaxPacketSize = 1500
	)

	A2sPlayer := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x55, 0xFF, 0xFF, 0xFF, 0xFF}
	A2sInfo := []byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0x54,
		0x53, 0x6F, 0x75, 0x72, 0x63, 0x65, 0x20, 0x45, 0x6E, 0x67, 0x69, 0x6E, 0x65, 0x20,
		0x51, 0x75, 0x65, 0x72, 0x79, 0x00,
		0xFF, 0xFF, 0xFF, 0xFF}

	conn, err := net.DialTimeout("udp",
		fmt.Sprintf("%s:%d", conf.Host, conf.QueryPort),
		time.Duration(connectTimeout)*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(time.Duration(1) * time.Second))

	_, _ = conn.Write(A2sPlayer)

	var buf [MaxPacketSize]byte
	numread, err := conn.Read(buf[:MaxPacketSize])
	if err != nil {
		return nil, err
	}

	if 0x41 == buf[4] {
		copy(A2sPlayer[5:9], buf[5:9])

		_ = conn.SetDeadline(time.Now().Add(time.Duration(1) * time.Second))
		_, _ = conn.Write(A2sPlayer)

		numread, err = conn.Read(buf[:MaxPacketSize])
		if err != nil || 0x44 != buf[4] {
			return nil, err
		}
	}

	unparsed := make([]byte, numread)
	copy(unparsed, buf[:numread])

	unparsed = unparsed[4:]
	numplayers := int(unparsed[1])

	players := map[string]time.Duration{}

	startidx := 3
	var b []byte
	for i := 0; i < numplayers; i++ {
		// there is no ternary operator in GO
		if i == 0 {
			b = unparsed[startidx:]
		} else {
			b = b[startidx+1:]
		}
		stringTerminator := bytes.IndexByte(b, 0x00)
		if stringTerminator < 0 {
			return nil, nil
		}
		playerName := string(b[:stringTerminator]) // string (variable length)
		// ignore score
		duration := b[stringTerminator+5 : stringTerminator+9] // time, float (4 bytes)
		startidx = stringTerminator + 9

		onlineTime := time.Duration(int64(math.Float32frombits(binary.LittleEndian.Uint32(duration)))) * time.Second
		players[playerName] = onlineTime
	}

	_ = conn.SetDeadline(time.Now().Add(time.Duration(1) * time.Second))
	_, _ = conn.Write(A2sInfo)
	numread, err = conn.Read(buf[:MaxPacketSize])
	if err != nil {
		return players, nil
	}

	unparsed = make([]byte, numread)
	copy(unparsed, buf[:numread])

	idx := 6

	serverInfo.name, idx = readCStringFromByteArray(unparsed, idx)
	serverInfo.srvmap, idx = readCStringFromByteArray(unparsed, idx)
	_, idx = readCStringFromByteArray(unparsed, idx) // folder
	_, idx = readCStringFromByteArray(unparsed, idx) // game name
	idx += 2

	serverInfo.players = int(unparsed[idx])
	idx++
	serverInfo.maxPlayers = int(unparsed[idx])
	idx++
	serverInfo.bots = int(unparsed[idx])
	idx++

	return players, nil
}

func readCStringFromByteArray(buffer []byte, startIndex int) (string, int) {
	if startIndex >= len(buffer) {
		return "", -1
	}

	stringTerminator := bytes.IndexByte(buffer[startIndex:], 0x00)
	if stringTerminator < 0 {
		return "", -1
	}

	str := string(buffer[startIndex : startIndex+stringTerminator])
	return str, stringTerminator + startIndex + 1
}

func nextTime(delay int) time.Time {
	return time.Now().Add(time.Duration(delay) * time.Second)
}

func rconSay(msg string) {
	conn, err := net.DialTimeout("tcp",
		fmt.Sprintf("%s:%d", conf.Host, conf.RconPort),
		time.Duration(connectTimeout)*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	const (
		cmdAuth        = 3
		cmdExecCommand = 2
		authResponse   = 2
	)

	buffer, connectId := makeRconBuffer(conf.RconPassword, cmdAuth)

	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(buffer)
	if err != nil {
		return
	}

	recvBuffer := make([]byte, 14)

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	size, err := conn.Read(recvBuffer)
	if size < 10 {
		s, err := conn.Read(recvBuffer[size:])
		if err != nil {
			return
		}
		size += s
	}

	if err != nil || size != 14 {
		return
	}

	if authResponse != binary.LittleEndian.Uint32(recvBuffer[8:12]) ||
		connectId != int32(binary.LittleEndian.Uint32(recvBuffer[4:8])) {
		log.Fatalln("ERROR: RCON authentification failed. Password wrong?")
		// Fatal = os.Exit(1)
	}

	buffer, connectId = makeRconBuffer("say "+msg, cmdExecCommand)
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(buffer)
	if err != nil {
		return
	}
}

func makeRconBuffer(msg string, cmdId int32) ([]byte, int32) {
	connectId := int32((time.Now().UnixNano() / 100000) % 100000)

	buffer := bytes.NewBuffer(make([]byte, 0, len(msg)+14))

	_ = binary.Write(buffer, binary.LittleEndian, int32(10+len(msg)))
	_ = binary.Write(buffer, binary.LittleEndian, connectId)
	_ = binary.Write(buffer, binary.LittleEndian, cmdId)

	buffer.WriteString(msg)
	_ = binary.Write(buffer, binary.LittleEndian, uint16(0))

	return buffer.Bytes(), connectId
}

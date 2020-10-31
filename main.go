package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

var (
	Token      string
	DBFilepath string = "campaigns.db"
	DB *sql.DB
)

func init() {
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

func main() {
	if !dbExists(DBFilepath) {
		file, err := os.Create(DBFilepath)
		if err != nil {
			fmt.Println("error creating Database:\n", err)
			return
		}
		file.Close()
		fmt.Println("Database created at ", DBFilepath)
	}

	db, err := sql.Open("sqlite3", DBFilepath)
	DB = db
	defer DB.Close()
	if err != nil {
		fmt.Println("error opening the Database:\n", err)
		return
	}

	statement, err := DB.Prepare(`CREATE TABLE IF NOT EXISTS campaigns (id TEXT PRIMARY KEY, player_pool INTEGER, dm_pool INTEGER)`)
	defer statement.Close()
	if err != nil {
		fmt.Println("Error preparing table create statement:\n", err)
		return
	}
	statement.Exec()

	dg, err := discordgo.New("Bot " + Token)
	defer dg.Close()
	if err != nil {
		fmt.Println("error creating Discord session:\n", err)
		return
	}

	dg.AddHandler(messageCreated)

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildMessages)

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func messageCreated(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID || !strings.HasPrefix(m.Content, "*mf") {
		return
	}

	args := strings.Split(m.Content, " ")

	if len(args) == 1 {
		s.ChannelMessageSend(m.ChannelID, "Hi!")
	}

	if args[1] == "help" {
		s.ChannelMessageSend(m.ChannelID, "Available commands:\n"+
			"- `*mf start <number>`    Create a Metal Factor dice pool with `<number>` in the player's pool\n"+
			"- `*mf check`    Check the contents of each pool\n"+
			"- `*mf pc <number>`    Remove `<number>` from the player's Metal Factor pool and adds it to the DM's\n"+
			"- `*mf dm <number>`    Remove `<number>` from the DM's Metal Factor pool and adds it to the player's\n"+
			"- `*mf set <player_value> <dm_value>`    Set the player's pool to `<player_value>` and the DM's pool to `<dm_value>`")
		return
	}

	if args[1] != "pc" &&
		args[1] != "dm" &&
		args[1] != "set" &&
		args[1] != "start" &&
		args[1] != "check" &&
		args[1] != "rickroll" {
		s.ChannelMessageSend(m.ChannelID, "Unknown command, type `*mf help` for a list of commands")
		return
	}

	switch args[1] {
	case "pc":
		fallthrough
	case "dm":
		if len(args) != 3 {
			s.ChannelMessageSend(m.ChannelID, "Malformed adjustment command, type `*mf help` for a list of commands")
			return
		}
		value, err := strconv.Atoi(args[2])
		if err != nil {
			fmt.Println(err)
			s.ChannelMessageSend(m.ChannelID, "Malformed adjustment command, type `*mf help` for a list of commands")
			return
		}
		newPlayerPool, newDMPool, err := updateMetalFactor(args[1], value, m.GuildID)
		s.ChannelMessageSend(m.ChannelID, "Metal Factor Updated!\n" +
					"The players have " + strconv.Itoa(newPlayerPool) + " dice in their pool\n" +
					"The DM has " + strconv.Itoa(newDMPool) + " dice in their pool")
		break

	case "set":
		if len(args) != 4 {
			s.ChannelMessageSend(m.ChannelID, "Malformed set command, type `*mf help` for a list of commands")
			return
		}
		playerValue, err := strconv.Atoi(args[2])
		dmValue, err := strconv.Atoi(args[3])
		if err != nil {
			fmt.Println(err)
			s.ChannelMessageSend(m.ChannelID, "Malformed set command, type `*mf help` for a list of commands")
			return 
		}

		err = setMetalFactor(playerValue, dmValue, m.GuildID)
		if err != nil {
			fmt.Println(err)
			s.ChannelMessageSend(m.ChannelID, "Ooopsie, something went wrong! I must have drunk too much rhum!")
			return
		}
		s.ChannelMessageSend(m.ChannelID, "The dice have been... not cast I guess, but placed!\n" +
				"The players have " + args[2] + " dice in their pool\n" +
				"The DM has " + args[3] + " dice in their pool")
		break

	case "start":
		if len(args) != 3 {
			s.ChannelMessageSend(m.ChannelID, "Malformed start command, type `*mf help` for a list of commands")
			return
		}
		startingValue, err := strconv.Atoi(args[2])
		err = createCampaign(startingValue, m.GuildID)
		if err != nil {
			fmt.Println(err)
			s.ChannelMessageSend(m.ChannelID, "Malformed start command, type `*mf help` for a list of commands")
			return
		}
		s.ChannelMessageSend(m.ChannelID, 
			"Campaign created with " + args[2] + " dice in the player's pool\n" +
			"Adventure awaits! Go plunder that booty! 🏴‍☠️")
		break

	case "check":
		playerPool, dmPool, err := checkPools(m.GuildID)
		if err != nil {
			fmt.Println(err)
			s.ChannelMessageSend(m.ChannelID, "Ooopsie, something went wrong! I must have drunk too much rhum!")
			return
		}
		s.ChannelMessageSend(m.ChannelID, "PC " + strconv.Itoa(playerPool) + " | " + strconv.Itoa(dmPool) + " DM")
		break

	case "rickroll":
		s.ChannelMessageSend(m.ChannelID, "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
		break
	}
}

func dbExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func createCampaign(startingValue int, id string) error {
	statement, err := DB.Prepare(`INSERT OR REPLACE INTO campaigns (id, player_pool, dm_pool) VALUES (?, ?, ?)`)
	defer statement.Close()
	_, err = statement.Exec(id, startingValue, 0)
	return err
}

func updateMetalFactor(user string, amount int, id string) (int, int, error) {
	
	playerPool, dmPool, err := checkPools(id)
	if err != nil {
		return 0, 0, err
	}

	var statement *sql.Stmt
	statement, err = DB.Prepare(`
	UPDATE campaigns
	SET player_pool = ?,
	dm_pool = ?
	WHERE
	id = ?
	`)
	if err != nil {
		return 0, 0, err
	}
	defer statement.Close()

	if user == "pc" {
		playerPool -= amount
		dmPool += amount
	} else if user == "dm" {
		playerPool += amount
		dmPool -= amount
	}
	_, err = statement.Exec(playerPool, dmPool, id)
	
	return playerPool, dmPool, err
}

func checkPools(id string) (int, int, error) {
	rows, err := DB.Query(`SELECT player_pool, dm_pool FROM campaigns WHERE id = ?`, id)
	if err != nil {
		return 0, 0, err
	}
	var playerPool int
	var dmPool int
	for rows.Next() {
		rows.Scan(&playerPool, &dmPool)
	}
	rows.Close()
	return playerPool, dmPool, err
}

func setMetalFactor(playerValue int, dmValue int, id string) error {
	var statement *sql.Stmt
	statement, err := DB.Prepare(`
	UPDATE campaigns
	SET player_pool = ?,
	dm_pool = ?
	WHERE
	id = ?
	`)
	if err != nil {
		return err
	}
	defer statement.Close()
	
	statement.Exec(playerValue, dmValue, id)
	return err
}
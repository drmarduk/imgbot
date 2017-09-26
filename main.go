package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strings"

	"github.com/jzelinskie/geddit"
	"github.com/quiteawful/qairc"
)

const (
	useragent = "windows:subredditcrawler:dev (by /u/txtinput"
)

var (
	images map[string]map[string]int
)

type options struct {
	Nick    string
	User    string
	Channel string
	Network string
	Port    int

	Session *geddit.Session
}

func parseArgs() *options {
	a := &options{
		Nick:    "ImgBotdev",
		User:    "marduk",
		Channel: "ownbox",
		Network: "irc.quiteawful.net",
		Port:    6697,
	}

	flag.StringVar(&a.Nick, "nick", a.Nick, "nickname of the bot")
	flag.StringVar(&a.User, "user", a.User, "owner of the bot")
	flag.StringVar(&a.Channel, "channel", a.Channel, "channel we join the bot")
	flag.StringVar(&a.Network, "network", a.Network, "hostname of the net we want to join")
	flag.IntVar(&a.Port, "port", a.Port, "the port on we connect")
	flag.Parse()

	if a.Channel[0] != '#' {
		a.Channel = "#" + a.Channel
	}

	a.Session = geddit.NewSession(useragent)

	return a
}

func main() {
	args := parseArgs()

	images = make(map[string]map[string]int, 5)

	RunIrc(args)
}

// RunIrc starts the irc daemon
func RunIrc(opt *options) {
	ctx := qairc.QAIrc(opt.Nick, opt.User)
	ctx.Address = fmt.Sprintf("%s:%d", opt.Network, opt.Port)
	ctx.UseTLS = true
	ctx.TLSCfg = &tls.Config{InsecureSkipVerify: true}

	err := ctx.Run()
	if err != nil {
		log.Fatalf("error while running irc context: %v\n", err)
		return
	}

	for {
		m, status := <-ctx.Out
		if !status {
			ctx.Reconnect()
		}

		if m.Type == "001" {
			ctx.Join(opt.Channel)
			log.Printf("Joined: %s\n", opt.Channel)
		}

		if m.Sender.Nick == opt.Nick {
			continue
		}

		if m.Type == "PRIVMSG" {
			l := len(m.Args)
			msg := m.Args[l-1]
			sender := m.Sender.Nick

			if match, err := regexp.MatchString("^!(cat|boobs|imgbot)", msg); match && err == nil {
				log.Printf("Received %s: %s", sender, msg)
				sub := ""
				if strings.HasPrefix(msg, "!cat") {
					sub = "cat"
				}
				if strings.HasPrefix(msg, "!boobs") {
					sub = "boobs"
				}

				img := getRandomImage(opt, sub)
				resp := fmt.Sprintf("PRIVMSG %s :%s\r\n", sender, img)
				log.Printf("Send: %s", resp)
				ctx.In <- resp
			}
		}
	}
}

func getRandomImage(opt *options, reddit string) string {
	if len(images[reddit]) == 0 {
		fillCache(opt, reddit)
		// derp
		return getRandomImage(opt, reddit)
	}

	max := len(images[reddit])
	rnd := rand.Intn(max)

	i := 0
	for k, v := range images[reddit] {
		if rnd == i {
			delete(images[reddit], k)
			v-- // bogus
			return k
		}
		i++
	}
	return "nope"
}

func fillCache(opt *options, reddit string) {
	submissions, err := opt.Session.SubredditSubmissions(
		reddit,
		"new",
		geddit.ListingOptions{
			Count: 100,
		},
	)

	if err != nil {
		log.Printf("error while renewing cache: %v\n", err.Error())
	}

	images[reddit] = make(map[string]int)
	for _, post := range submissions {
		// TODO: add more domains
		if strings.Contains(post.URL, "i.redd.it") || strings.Contains(post.URL, "imgur.com") {
			images[reddit][post.URL] = 1
		}
	}
	log.Printf("Inserted %d from %d %s submissions\n", len(images[reddit]), len(submissions), reddit)
}

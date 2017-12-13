package niconico

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yhat/scrape"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	jst = time.FixedZone("Asia/Tokyo", 9*60*60)
)

type Comic struct {
	ID          string
	Title       string
	Author      string
	Start       time.Time
	Update      time.Time
	EpisodeList []struct {
		Title string
		URL   string
	}
}

func (c Comic) GetMainURL() string {
	if c.ID == "" {
		return ""
	}
	return "http://seiga.nicovideo.jp/comic/" + c.ID
}

func (c Comic) GetStartDateString() string {
	return c.Start.In(jst).Format("2006年01月02日")
}

func (c Comic) GetUpdateDateString() string {
	return c.Update.In(jst).Format("2006年01月02日")
}

func (c Comic) GetFirstEpisodeURL() string {
	if c.ID == "" {
		return ""
	}
	return c.GetMainURL() + "/ep1"
}

func (c Comic) GetLatestEpisodeURL() string {
	if c.ID == "" {
		return ""
	}
	return c.GetMainURL() + "/new"
}

func (c Comic) GetEpisodeCount() int {
	return len(c.EpisodeList)
}

func (c *Comic) Get(client *http.Client) error {
	if c.ID == "" {
		return fmt.Errorf("ID is empty")
	}
	resp, err := client.Get(c.GetMainURL())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http.StatusCode != http.StatusOK : %v", resp.StatusCode)
	}
	root, err := html.Parse(resp.Body)
	if err != nil {
		return err
	}

	if mainTitleNode, ok := scrape.Find(root, func(node *html.Node) bool {
		return scrape.Attr(node, "class") == "main_title"
	}); !ok {
		return fmt.Errorf("failed to find main title node")
	} else {
		if titleNode, ok := scrape.Find(mainTitleNode, func(node *html.Node) bool {
			return node.DataAtom == atom.H1
		}); ok {
			c.Title = scrape.Text(titleNode)
		} else {
			return fmt.Errorf("failed to find title node")
		}

		if authorNode, ok := scrape.Find(mainTitleNode, func(node *html.Node) bool {
			return node.DataAtom == atom.H3
		}); ok {
			c.Author = strings.Replace(scrape.Text(authorNode), "作者:", "", 1)
		} else {
			return fmt.Errorf("failed to find author node")
		}
	}

	if metaInfoNode, ok := scrape.Find(root, func(node *html.Node) bool {
		// meta_info クラスを持つ要素は複数あるが、欲しいノードは先に出現するのでこれでよしとする
		return scrape.Attr(node, "class") == "meta_info"
	}); !ok {
		return fmt.Errorf("failed to find meta info node")
	} else {
		metaInfoNodeText := scrape.Text(metaInfoNode)
		startDateMatchStrings := regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日開始`).FindStringSubmatch(metaInfoNodeText)
		year, _ := strconv.Atoi(startDateMatchStrings[1])
		month, _ := strconv.Atoi(startDateMatchStrings[2])
		date, _ := strconv.Atoi(startDateMatchStrings[3])
		c.Start = time.Date(year, time.Month(month), date, 0, 0, 0, 0, jst)

		updateDateMatchStrings := regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日更新`).FindStringSubmatch(metaInfoNodeText)
		year, _ = strconv.Atoi(updateDateMatchStrings[1])
		month, _ = strconv.Atoi(updateDateMatchStrings[2])
		date, _ = strconv.Atoi(updateDateMatchStrings[3])
		c.Update = time.Date(year, time.Month(month), date, 0, 0, 0, 0, jst)
	}

	if episodeListNode, ok := scrape.Find(root, func(node *html.Node) bool {
		return scrape.Attr(node, "id") == "episode_list"
	}); !ok {
		return fmt.Errorf("failed to find episode list node")
	} else {
		for _, n := range scrape.FindAll(episodeListNode, func(node *html.Node) bool {
			return node.DataAtom == atom.Li && scrape.Attr(node, "class") == "episode_item"
		}) {
			if a, ok := scrape.Find(n, func(node *html.Node) bool {
				return node.DataAtom == atom.A && node.Parent != nil && scrape.Attr(node.Parent, "class") == "title"
			}); !ok {
				return fmt.Errorf("failed to find title node in episode list")
			} else {
				c.EpisodeList = append(c.EpisodeList, struct {
					Title string
					URL   string
				}{Title: scrape.Text(a), URL: strings.TrimSuffix(scrape.Attr(a, "href"), "?track=ct_episode")})
			}
		}
	}
	return nil
}

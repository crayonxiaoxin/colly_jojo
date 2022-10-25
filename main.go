package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
)

type SaveFile struct {
	Url   string
	Index string
	Dir   string
}

var wp sync.WaitGroup

func main() {
	c := colly.NewCollector(
		colly.Async(true),
		colly.Debugger(&debug.LogDebugger{}),
	)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*manhua*",              // 匹配url：DomainGlob 和 DomainRegexp 至少填一项
		Parallelism: 2,                       // 并发数
		Delay:       500 * time.Millisecond,  // 基础间隔500毫秒
		RandomDelay: 1000 * time.Millisecond, // 最终间隔为500毫秒~1500毫秒之间
	})

	// 获取章节列表
	// 全部		.tab-pane[id=\"1328\"] .links-of-books.num_div>li>a[href]
	// 第1个	.tab-pane[id=\"1328\"] .links-of-books.num_div>li:first-child>a[href]
	// 第N个	.tab-pane[id=\"1328\"] .links-of-books.num_div>li:nth-child(2)>a[href]

	// JOJO 6
	// c.OnHTML(".tab-pane[id=\"1328\"] .links-of-books.num_div>li>a[href]", func(e *colly.HTMLElement) {
	// 	fmt.Printf("e.Attr(\"href\"): %v\n", e.Attr("href"))
	// 	e.Request.Visit(e.Attr("href"))
	// })

	// JOJO 7
	c.OnHTML(".tab-pane[id=\"1330\"] .links-of-books.num_div>li>a[href]", func(e *colly.HTMLElement) {
		fmt.Printf("e.Attr(\"href\"): %v\n", e.Attr("href"))
		e.Request.Visit(e.Attr("href"))
	})

	c.OnError(func(r *colly.Response, err error) {
		fmt.Printf("------ 响应错误: %v\n", err)
	})

	// c.OnRequest(func(r *colly.Request) {
	// 	rand.Seed(time.Now().UnixNano())
	// 	value := rand.Intn(10) + 5 // 5~15 随机数
	// 	value = value * 100
	// 	fmt.Printf("delay: %v\n", value)
	// 	time.Sleep(time.Duration(value) * time.Millisecond)
	// })

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	// 获取漫画内容
	c.OnHTML(".comic-detail", func(e *colly.HTMLElement) {
		// 当前集数
		vol := e.ChildText("h2.h4.text-center")
		fmt.Printf("集数: %v\n", vol)

		// 当前图片
		img := e.ChildAttr("#all .pjax-container img.img-fluid.show-pic", "src")
		fmt.Printf("图片: %v\n", img)

		// 下一页页码
		nextPage := e.ChildAttr("#all a#right.next", "data-p")
		fmt.Printf("下一页: %v\n", nextPage)

		// 当前url
		u := e.Response.Request.URL.String()

		// 当前页码
		r, err := regexp.Compile("_p\\d+.html")
		currentPage := ""
		if err != nil {
			fmt.Printf("err: %v\n", err)
		} else {
			s := r.FindString(u)
			fmt.Printf("s: %v\n", s)
			s = strings.Replace(s, "_p", "", 1)
			s = strings.Replace(s, ".html", "", 1)
			currentPage = s
		}
		if currentPage == "" {
			currentPage = "1"
		}
		fmt.Printf("当前页: %v\n", currentPage)

		if img == "" {
			return
		}

		// 保存图片
		wp.Add(1)
		go save_file(SaveFile{
			Url:   img,
			Index: currentPage,
			Dir:   vol,
		})

		// 访问下一页
		if nextPage != "" && nextPage != "0" {
			fmt.Printf("u: %v\n", u)
			s := strings.Split(u, "_")
			if len(s) == 2 {
				// 第一页的情况下，url中可能没有页码 _p1
				s2 := strings.Split(u, ".")
				s2[len(s2)-2] += "_p" + nextPage
				nextUrl := strings.Join(s2, ".")
				fmt.Printf("nextUrl: %v\n", nextUrl)
				e.Request.Visit(nextUrl)
			} else if len(s) == 3 {
				s[2] = "p" + nextPage + ".html"
				nextUrl := strings.Join(s, "_")
				fmt.Printf("nextUrl: %v\n", nextUrl)
				e.Request.Visit(nextUrl)
			}
		}
	})

	// c.Visit("https://www.manhuadb.com/manhua/139") // JOJO 6 石之海
	// c.Visit("https://www.manhuadb.com/manhua/147") // JOJO 7 飙马野郎
	// c.Visit("https://www.manhuadb.com/manhua/154") // JOJO 8 jojo福音

	c.Wait()  // 等待所有请求完成
	wp.Wait() // 等待所有协程（文件保存）完成
}

func save_file(sf SaveFile) {
	fmt.Printf("sf: %v\n", sf)
	r, err := http.Get(sf.Url)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	} else {
		defer r.Body.Close()
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Printf("err: %v\n", err)
		} else {
			s := strings.Split(sf.Url, ".")
			mime := s[len(s)-1]
			if mime == "" {
				mime = "jpg"
			}
			os.Mkdir(sf.Dir, 0777)

			filename := sf.Dir + "/" + sf.Index + "." + mime
			fmt.Printf("filename: %v\n", filename)

			err := ioutil.WriteFile(filename, b, 0777)
			if err != nil {
				fmt.Printf("err: %v\n", err)
			}
		}
	}
	wp.Done()
}

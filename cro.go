package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"time"
	//iconv "github.com/djimenez/iconv-go"

	"bytes"
	"log"
	"os"
	"regexp"
	"text/template"

	"github.com/urfave/cli"
)

type CroProgram struct {
	Data []struct {
		Description string    `json:"description"`
		ID          int       `json:"id"`
		Since       time.Time `json:"since"`
		Station     string    `json:"station"`
		Till        time.Time `json:"till"`
		Title       string    `json:"title"`
	} `json:"data"`
	Timestamp string `json:"timestamp"`
}

func main() {
	var url string
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "url",
			Value:       "http://api.rozhlas.cz/data/v2/schedule/day/2016/06/04/vltava.json",
			Usage:       "--url http://api.rozhlas.cz/data/v2/schedule/day/2016/06/04/vltava.json",
			Destination: &url,
		},
	}
	app.Action = func(c *cli.Context) error {
		resp, err := http.Get(url)
		if err != nil {
			fmt.Println("error:", err)
		}
		defer resp.Body.Close()
		resp_body, err := ioutil.ReadAll(resp.Body) //<--- here!

		if err != nil {
			fmt.Println(err)
		}
		//	resp_body, _ := httputil.DumpResponse(resp, true)
		//fmt.Printf("%s", resp_body)

		var croprg CroProgram
		err = json.Unmarshal(resp_body, &croprg)
		if err != nil {
			fmt.Println("error:", err)
		}
		//fmt.Printf("%v", croprg)
		const at_job = `
FILE='{{.File}}'
I=0; while [ -f "$FILE" ]; do  I=$((I+1)); FILE="$FILE-$I"; done
mplayer {{.Stream}} -dumpstream -dumpfile "$FILE" &> /dev/null &
sleep {{.Duration}}
kill %%
vorbiscomment -a "$FILE" -t 'TITEL={{.Title}}' -t 'COMMENT={{.Comment}}'
`
		tmpl := template.Must(template.New("atjob").Parse(at_job))

		for i, data := range croprg.Data {
			if matched, _ := regexp.MatchString("^(Rozhlasov. (jevi|hra)|Hra pro tento ve|Seri.l|.etba na pokra)", croprg.Data[i].Title); matched {
				//b, _ := json.MarshalIndent(croprg.Data[i], "", "  ")
				//_,_ := iconv.ConvertString(string(b), "utf-8", "ascii//TRANSLIT")

				t := data.Since

				if t.After(time.Now()) { // je budouci programy
					file := fmt.Sprintf("%s.%04d%02d%02d%02d%02d.ogg", data.Title, t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())
					// at job budeme vytvaret je pro aktualni den
					hourminutes := fmt.Sprintf("%02d%02d", t.Hour(), t.Minute())
					// trvani nahravky v sek. + 1 minuta
					duration := fmt.Sprintf("%.f", data.Till.Sub(data.Since).Seconds()+60)
					re := regexp.MustCompile("['/: ]")
					file = re.ReplaceAllString(file, "_")
					//data.Description = re.ReplaceAllString( data.Description,"-")
					var stream string
					switch data.Station {
					case "vltava":
						stream = "http://amp.cesnet.cz:8000/cro3-256.ogg"
					case "dvojka":
						stream = "http://amp.cesnet.cz:8000/cro2-256.ogg"

					}

					type Atjob struct {
						File, Hourminutes, Duration, Stream, Title, Comment string
					}
					var doc bytes.Buffer
					err := tmpl.Execute(&doc, Atjob{file, hourminutes, duration, stream, data.Title, data.Description})
					if err != nil {
						fmt.Println("executing template:", err)
					}

					cmd := exec.Command("at", hourminutes) // pouze parsing prikazu, spusteni potom pomoci cmd.Run
					cmd.Stdin = &doc
					var out bytes.Buffer
					cmd.Stdout = &out
					err = cmd.Run()
					if err != nil {
						log.Fatal(err)
					}
				}
				//fmt.Printf("%s\n", croprg.Data[i].Description)
			}
		}
		return nil
	}
	app.Run(os.Args)
}

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"wtdata/reader"
	"wtdata/internal/types"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	sub := os.Args[1]
	switch sub {
	case "hot-build":
		cmdHotBuild(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Println("wtdata CLI")
	fmt.Println("  hot-build   Build combined hot .dsb from sections config")
}

func cmdHotBuild(args []string) {
	fs := flag.NewFlagSet("hot-build", flag.ExitOnError)
	var (
		base   = fs.String("base", "./", "base directory (contains his/rt)")
		cfg    = fs.String("config", "", "hot sections json config path")
		exchg  = fs.String("exchg", "", "exchange, e.g. SHFE")
		product = fs.String("product", "", "product, e.g. rb")
		rule   = fs.String("rule", "main", "custom rule name")
		period = fs.String("period", "min1", "period: min1|min5|day")
		exright = fs.String("exright", "", "exright: '', '-'(QFQ), '+'(HFQ)")
	)
	_ = fs.Parse(args)
	if *cfg == "" || *exchg == "" || *product == "" {
		log.Fatal("config/exchg/product required")
	}
	var p int
	switch *period {
	case "min1": p = types.KP_Minute1
	case "min5": p = types.KP_Minute5
	case "day":  p = types.KP_DAY
	default:
		log.Fatalf("invalid period: %s", *period)
	}
	var xr byte
	if *exright == "-" { xr = byte(types.SUFFIX_QFQ) }
	if *exright == "+" { xr = byte(types.SUFFIX_HFQ) }
	r := &reader.Reader{}
	r.Init(*base, "", 0)
	if err := r.LoadHotSectionsFromFile(*cfg); err != nil { log.Fatal(err) }
	if err := r.BuildHotCombined(*exchg, *product, *rule, p, xr); err != nil { log.Fatal(err) }
}
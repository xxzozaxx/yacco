package main

import (
	"fmt"
	"path/filepath"
	sysre "regexp"
	"runtime"
	"strings"

	"github.com/aarzilli/yacco/config"
	"github.com/aarzilli/yacco/edit"
	"github.com/aarzilli/yacco/regexp"
	"github.com/aarzilli/yacco/util"
)

type LoadRule struct {
	ForDir bool
	BufRe  *sysre.Regexp
	Re     *regexp.Regex
	Action string
}

var LoadRules []LoadRule

func LoadInit() {
	LoadRules = []LoadRule{}
	for _, rule := range config.LoadRules {
		if (rule.Action[0] != 'L') && (rule.Action[0] != 'X') {
			panic(fmt.Errorf("Actions must start with X or L in: %s", rule.Action))
		}
		var bufRe *sysre.Regexp = nil
		if rule.BufRe != "/" {
			bufRe = sysre.MustCompile(rule.BufRe)
		} else {
			bufRe = nil
		}
		LoadRules = append(LoadRules, LoadRule{ForDir: bufRe == nil, BufRe: bufRe, Re: regexp.Compile(rule.Re, true, false), Action: rule.Action})
	}
	config.StartupWidth = config.MainFontSize * 40
	config.StartupHeight = config.MainFontSize * 30
	config.ScrollWidth = int(float64(config.MainFontSize) * 0.625)
	config.ComplMaxX = config.MainFontSize * 60
	config.ComplMaxY = config.MainFontSize * 60
}

func printStackTrace() {
	for i := 1; ; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		fmt.Printf("  %s:%d\n", file, line)
	}
}

func Load(ec ExecContext, origin int) {
	defer func() {
		if ierr := recover(); ierr != nil {
			fmt.Printf("Error during Load: %v\n", ierr.(error).Error())
			printStackTrace()
		}
	}()
	//println("\nin load")
	if ec.buf == nil {
		return
	}
	for _, rule := range LoadRules {
		path := filepath.Join(ec.buf.Dir, ec.buf.Name)
		if rule.ForDir {
			if !ec.buf.IsDir() {
				continue
			}
		} else {
			if !rule.BufRe.MatchString(path) {
				continue
			}
		}
		start := ec.fr.Sel.S
		for {
			matches := rule.Re.Match(ec.buf, start, ec.fr.Sel.E, +1)
			//println("match:", matches, ec.fr.Sels[2].S, ec.fr.Sels[2].E)
			if matches == nil {
				break
			}
			s := matches[0]
			e := matches[1]

			//println("match:", s, e, origin)

			ok := false
			if origin < 0 {
				ok = (s == ec.fr.Sel.S) && (e == ec.fr.Sel.E)
			} else {
				ok = (s <= origin) && (origin <= e)
			}

			if ok {
				strmatches := []string{}
				for i := 0; 2*i+1 < len(matches); i++ {
					s := matches[2*i]
					e := matches[2*i+1]
					strmatches = append(strmatches, string(ec.buf.SelectionRunes(util.Sel{s, e})))
				}
				//println("Match:", strmatches[0])
				if rule.Exec(ec, strmatches, s, e) {
					return
				} else {
					// abandon rule after the first match straddling the origin
					break
				}
			}

			start = s + 1
			if start > origin {
				break
			}
		}
	}
}

func expandMatches(str string, matches []string) string {
	out := []byte{}
	sub := false
	tolower := false
	for i := range str {
		if !sub {
			if str[i] == '$' {
				tolower = false
				sub = true
			} else {
				out = append(out, str[i])
			}
		} else {
			if str[i] == 'l' {
				tolower = true
			} else if (str[i] >= '0') && (str[i] <= '9') {
				d := int(str[i] - '0')
				if d >= len(matches) {
					out = append(out, '$')
					out = append(out, str[i])
				} else {
					if tolower {
						out = append(out, strings.ToLower(matches[d])...)
					} else {
						out = append(out, matches[d]...)
					}
				}
				sub = false
			} else {
				out = append(out, '$')
				out = append(out, str[i])
				sub = false
			}
		}
	}
	return string(out)
}

func (rule *LoadRule) Exec(ec ExecContext, matches []string, s, e int) bool {
	defer func() {
		if ierr := recover(); ierr != nil {
			fmt.Printf("Error during Load (exec): %v\n", ierr.(error).Error())
			printStackTrace()
		}
	}()
	action := rule.Action[1:]

	switch rule.Action[0] {
	case 'X':
		expaction := expandMatches(action, matches)
		ec.fr.Sel = util.Sel{s, e}
		ec.fr.SelColor = 2
		Exec(ec, expaction)
		return true
	case 'L':
		v := strings.SplitN(action, ":", 2)
		name := expandMatches(v[0], matches)

		addrExpr := ""
		if len(v) > 1 {
			addrExpr = expandMatches(v[1], matches)
		}
		var newed *Editor
		if name != "" {
			var err error
			newed, err = EditFind(ec.dir, name, false, false)
			if err != nil {
				return false
			}
			if newed == nil {
				return false
			}
		} else {
			newed = ec.ed
		}
		ec.fr.Sel = util.Sel{s, e}
		ec.fr.SelColor = 2
		ec.br()
		if addrExpr != "" {
			func() {
				defer func() {
					recover()
					// do nothing, doesn't matter anyway
				}()
				newed.sfr.Fr.SelColor = 0
				newed.sfr.Fr.Sel = util.Sel{0, 0}
				newed.sfr.Fr.Sel = edit.AddrEval(addrExpr, newed.bodybuf, newed.sfr.Fr.Sel)
			}()
			newed.BufferRefresh()
		}
		newed.Warp()
		return true
	}
	return false
}

package documents_test

import (
	"bytes"
	"compress/zlib"
	"io"
	"math"
	"regexp"
	"strconv"
	"testing"
)

// Kleiner PDF-Leser für Tests: liefert je Seite die entpackten Zeichenbefehle.
// Reicht für die Ausgabe von Chromium und Gotenbergs Zusammenfügen (keine Objekt-Streams).

var (
	objStart  = regexp.MustCompile(`(?m)(?:^|[\r\n])(\d+) 0 obj\b`)
	refRe     = regexp.MustCompile(`(\d+) 0 R`)
	kidsRe    = regexp.MustCompile(`/Kids\s*\[([^\]]*)\]`)
	contentRe = regexp.MustCompile(`/Contents\s*(\[[^\]]*\]|\d+ 0 R)`)
	typePages = regexp.MustCompile(`/Type\s*/Pages\b`)
	typePage  = regexp.MustCompile(`/Type\s*/Page\b`)
)

type pdfObject struct {
	dict   []byte // alles vor "stream" bzw. der ganze Inhalt
	stream []byte // entpackter Stream oder nil
}

// pdfObjects liest alle "N 0 obj … endobj"-Objekte.
func pdfObjects(t *testing.T, pdf []byte) map[int]pdfObject {
	t.Helper()
	objs := map[int]pdfObject{}
	locs := objStart.FindAllSubmatchIndex(pdf, -1)
	for i, loc := range locs {
		num, _ := strconv.Atoi(string(pdf[loc[2]:loc[3]]))
		end := len(pdf)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		body := pdf[loc[1]:end]
		if e := bytes.LastIndex(body, []byte("endobj")); e >= 0 {
			body = body[:e]
		}
		obj := pdfObject{dict: body}
		if s := bytes.Index(body, []byte("stream")); s >= 0 && !bytes.HasPrefix(body[s:], []byte("streamx")) {
			obj.dict = body[:s]
			data := bytes.TrimLeft(body[s+len("stream"):], "\r\n")
			if e := bytes.LastIndex(data, []byte("endstream")); e >= 0 {
				data = bytes.TrimRight(data[:e], "\r\n")
			}
			obj.stream = data
			if bytes.Contains(obj.dict, []byte("/FlateDecode")) {
				r, err := zlib.NewReader(bytes.NewReader(data))
				if err == nil {
					obj.stream, _ = io.ReadAll(r) // ein abgeschnittenes Ende ist hier egal
				}
			}
		}
		objs[num] = obj
	}
	return objs
}

// pageContents liefert die Zeichenbefehle jeder Seite in Seitenreihenfolge.
func pageContents(t *testing.T, pdf []byte) [][]byte {
	t.Helper()
	objs := pdfObjects(t, pdf)
	root := -1
	for num, o := range objs {
		if typePages.Match(o.dict) && !bytes.Contains(o.dict, []byte("/Parent")) {
			root = num
		}
	}
	if root < 0 {
		t.Fatal("pdf: kein Seitenbaum gefunden")
	}
	var pages [][]byte
	var walk func(num int)
	walk = func(num int) {
		o := objs[num]
		switch {
		case typePages.Match(o.dict):
			if m := kidsRe.FindSubmatch(o.dict); m != nil {
				for _, r := range refRe.FindAllSubmatch(m[1], -1) {
					n, _ := strconv.Atoi(string(r[1]))
					walk(n)
				}
			}
		case typePage.Match(o.dict):
			var content []byte
			if m := contentRe.FindSubmatch(o.dict); m != nil {
				for _, r := range refRe.FindAllSubmatch(m[1], -1) {
					n, _ := strconv.Atoi(string(r[1]))
					content = append(content, objs[n].stream...)
					content = append(content, '\n')
				}
			}
			pages = append(pages, content)
		}
	}
	walk(root)
	return pages
}

// sidebarRGB ist die Farbe der Seitenleiste (#12343b), wie Chromium sie schreibt (".0706 .2039 .2314 rg").
var sidebarRGB = [3]float64{0x12 / 255.0, 0x34 / 255.0, 0x3b / 255.0}

// sidebarFilled meldet, ob die Seite ein gefülltes Rechteck in der Seitenleistenfarbe zeichnet,
// das am linken Rand beginnt und deutlich höher als breit ist – also die Seitenleiste.
func sidebarFilled(content []byte) bool {
	type rect struct{ x, w, h float64 }
	var (
		nums  []float64
		fill  [3]float64
		saved [][3]float64
		path  []rect
	)
	isSide := func(c [3]float64) bool {
		for i := range c {
			if math.Abs(c[i]-sidebarRGB[i]) > 0.005 {
				return false
			}
		}
		return true
	}
	for _, tok := range bytes.Fields(content) {
		if v, err := strconv.ParseFloat(string(tok), 64); err == nil {
			nums = append(nums, v)
			continue
		}
		switch string(tok) {
		case "q":
			saved = append(saved, fill)
		case "Q":
			if n := len(saved); n > 0 {
				fill, saved = saved[n-1], saved[:n-1]
			}
		case "rg":
			if n := len(nums); n >= 3 {
				fill = [3]float64{nums[n-3], nums[n-2], nums[n-1]}
			}
		case "re":
			if n := len(nums); n >= 4 {
				path = append(path, rect{x: nums[n-4], w: math.Abs(nums[n-2]), h: math.Abs(nums[n-1])})
			}
		case "f", "F", "f*", "B", "B*", "b", "b*":
			if isSide(fill) {
				for _, r := range path {
					if math.Abs(r.x) < 1 && r.w > 0 && r.h > 3*r.w {
						return true
					}
				}
			}
			path = nil
		case "n", "S", "s":
			path = nil
		}
		nums = nums[:0]
	}
	return false
}

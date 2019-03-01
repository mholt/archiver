package archiver

import (
	"errors"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

var encodings = map[string]encoding.Encoding{
	"ibm866":            charmap.CodePage866,
	"iso8859_2":         charmap.ISO8859_2,
	"iso8859_3":         charmap.ISO8859_3,
	"iso8859_4":         charmap.ISO8859_4,
	"iso8859_5":         charmap.ISO8859_5,
	"iso8859_6":         charmap.ISO8859_6,
	"iso8859_7":         charmap.ISO8859_7,
	"iso8859_8":         charmap.ISO8859_8,
	"iso8859_8I":        charmap.ISO8859_8I,
	"iso8859_10":        charmap.ISO8859_10,
	"iso8859_13":        charmap.ISO8859_13,
	"iso8859_14":        charmap.ISO8859_14,
	"iso8859_15":        charmap.ISO8859_15,
	"iso8859_16":        charmap.ISO8859_16,
	"koi8r":             charmap.KOI8R,
	"koi8u":             charmap.KOI8U,
	"macintosh":         charmap.Macintosh,
	"windows874":        charmap.Windows874,
	"windows1250":       charmap.Windows1250,
	"windows1251":       charmap.Windows1251,
	"windows1252":       charmap.Windows1252,
	"windows1253":       charmap.Windows1253,
	"windows1254":       charmap.Windows1254,
	"windows1255":       charmap.Windows1255,
	"windows1256":       charmap.Windows1256,
	"windows1257":       charmap.Windows1257,
	"windows1258":       charmap.Windows1258,
	"macintoshcyrillic": charmap.MacintoshCyrillic,
	"gbk":               simplifiedchinese.GBK,
	"gb18030":           simplifiedchinese.GB18030,
	"big5":              traditionalchinese.Big5,
	"eucjp":             japanese.EUCJP,
	"iso2022jp":         japanese.ISO2022JP,
	"shiftjis":          japanese.ShiftJIS,
	"euckr":             korean.EUCKR,
	"utf16be":           unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
	"utf16le":           unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM),
}

func GetEncoding(charset string) (encoding.Encoding, bool) {
	charset = strings.ToLower(charset)
	enc, ok := encodings[charset]
	return enc, ok
}

func Decode(in []byte, charset string) ([]byte, error) {
	if enc, ok := GetEncoding(charset); ok {
		return enc.NewDecoder().Bytes(in)
	}
	return nil, errors.New("charset not found!")
}

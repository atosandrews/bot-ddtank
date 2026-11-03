package headless

import "encoding/xml"

type loginResult struct {
	XMLName  xml.Name `xml:"Result"`
	Accepted bool     `xml:"value,attr"`
	Items    []struct {
		ID int32 `xml:"ID,attr"`
	} `xml:"Item"`
}

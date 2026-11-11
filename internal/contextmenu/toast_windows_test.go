package contextmenu

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestToastContentAndNativeConstruction(t *testing.T) {
	title, message := "讣告：完成 😀", `C:\文件 <>& ' " %PATH% 😀.txt`
	text := toastXML(title, message)
	var content struct {
		Duration string   `xml:"duration,attr"`
		Text     []string `xml:"visual>binding>text"`
		Audio    struct {
			Silent string `xml:"silent,attr"`
		} `xml:"audio"`
	}
	if err := xml.Unmarshal([]byte(text), &content); err != nil {
		t.Fatal(err)
	}
	if content.Duration != "short" || content.Audio.Silent != "true" || len(content.Text) != 2 || content.Text[0] != title || content.Text[1] != message {
		t.Fatal(content)
	}
	if err := withWinRT(func() error {
		toast, err := makeToast(text) // do not Show: tests never notify the real desktop
		if err == nil {
			toast.release()
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(shortText(strings.Repeat("😀", 1000)), "…") {
		t.Fatal("unbounded toast")
	}
}

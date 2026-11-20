package i18n

import "testing"

func TestResolveLanguage(t *testing.T) {
	getenv := func(string) string { return "" }
	for _, tc := range []struct {
		setting, system string
		want            Language
	}{
		{Auto, "zh-CN", Chinese}, {Auto, "zh-TW", English}, {Auto, "ZH-Hant-HK", English},
		{Auto, "en-US", English}, {Auto, "ja-JP", English}, {Auto, "", English},
		{"en", "zh-CN", English}, {"zh-CN", "en-US", Chinese},
	} {
		t.Run(tc.setting+"/"+tc.system, func(t *testing.T) {
			calls := 0
			got := ResolveWith(tc.setting, getenv, func() string { calls++; return tc.system })
			if got != tc.want || (calls == 1) != (tc.setting == Auto) {
				t.Fatalf("language=%s system calls=%d", got, calls)
			}
		})
	}
}

func TestResolveWithPrecedenceAndFallback(t *testing.T) {
	values := map[string]string{"OBIT_LANG": "en", "LC_ALL": "zh-CN"}
	getenv := func(name string) string { return values[name] }
	if got := ResolveWith("zh-CN", getenv, func() string { return "zh-CN" }); got != English {
		t.Fatalf("OBIT_LANG did not win: %s", got)
	}
	values["OBIT_LANG"] = ""
	if got := ResolveWith("zh-CN", getenv, func() string { return "en-US" }); got != Chinese {
		t.Fatalf("config did not win: %s", got)
	}
	if got := ResolveWith(Auto, func(name string) string {
		if name == "LC_ALL" {
			return "C"
		}
		if name == "LC_MESSAGES" {
			return "POSIX"
		}
		return "ja-JP"
	}, func() string { return "zh-TW" }); got != English {
		t.Fatalf("unsupported locales did not fall back to English: %s", got)
	}
}

package i18n

import "testing"

func TestResolveLanguage(t *testing.T) {
	for _, tc := range []struct {
		setting, system string
		want            Language
	}{
		{Auto, "zh-CN", Chinese}, {Auto, "zh-TW", Chinese}, {Auto, "ZH-Hant-HK", Chinese},
		{Auto, "en-US", English}, {Auto, "ja-JP", English}, {Auto, "", English},
		{"en", "zh-CN", English}, {"zh-CN", "en-US", Chinese},
	} {
		t.Run(tc.setting+"/"+tc.system, func(t *testing.T) {
			calls := 0
			got := Resolve(tc.setting, func() string { calls++; return tc.system })
			if got != tc.want || (calls == 1) != (tc.setting == Auto) {
				t.Fatalf("language=%s system calls=%d", got, calls)
			}
		})
	}
}

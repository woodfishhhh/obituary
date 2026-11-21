package i18n

import "testing"

func TestDoctorJSONKeepsHistoricalStatus(t *testing.T) {
	const key = "cli.init.integration_missing"
	if got := NewProtocolLocalizer().Text(key, nil); got != "未安装 (integration is missing)" {
		t.Fatalf("wire detail changed: %q", got)
	}
	if got := NewLocalizer(English).Text(key, nil); got != "integration is missing" {
		t.Fatalf("human detail changed: %q", got)
	}
}

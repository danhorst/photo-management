package photos

import "testing"

func TestResolveImportReport(t *testing.T) {
	report := []byte(`[
		{"filename":"good.heic","imported":true,"error":false,"uuid":"UUID-1"},
		{"filename":"bad.heic","imported":false,"error":true,"uuid":""},
		{"filename":"phantom.heic","imported":false,"error":false,"uuid":"UUID-2"}
	]`)
	paths := []string{
		"/Export/2005/06/good.heic",
		"/Export/2005/06/bad.heic",
		"/Export/2005/06/phantom.heic",
		"/Export/2005/06/missing.heic",
	}
	res, err := resolveImportReport(report, paths)
	if err != nil {
		t.Fatal(err)
	}
	if r := res["/Export/2005/06/good.heic"]; r.Err != nil || r.UUID != "UUID-1" {
		t.Errorf("imported file: got uuid=%q err=%v, want UUID-1/nil", r.UUID, r.Err)
	}
	if r := res["/Export/2005/06/bad.heic"]; r.Err == nil {
		t.Error("errored file must carry an error, not a uuid")
	}
	// The phantom case: a uuid present but imported=false must NOT count as success.
	if r := res["/Export/2005/06/phantom.heic"]; r.Err == nil {
		t.Error("a uuid without imported=true must be treated as a failure")
	}
	if r := res["/Export/2005/06/missing.heic"]; r.Err == nil {
		t.Error("a path absent from the report must carry an error")
	}
}

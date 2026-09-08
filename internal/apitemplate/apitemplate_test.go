package apitemplate

import (
	"strings"
	"testing"
)

func TestRender_NoDynamicParams(t *testing.T) {
	out := Render("modules/demo/api/v1/widget.py")
	if strings.Contains(out, "identifier") {
		t.Fatalf("no params expected:\n%s", out)
	}
	if !strings.Contains(out, "def get(self, request: BaseRequest):") {
		t.Fatalf("expected param-less get signature:\n%s", out)
	}
}

func TestRender_WithDynamicParams(t *testing.T) {
	out := Render("modules/hr/core/api/v1/employee/from_aadhaar/<str:identifier>/verify.py")
	if !strings.Contains(out, "def get(self, request: BaseRequest, identifier: str):") {
		t.Fatalf("expected identifier param in get signature:\n%s", out)
	}
	if !strings.Contains(out, "def post(self, request: BaseRequest, identifier: str):") {
		t.Fatalf("expected identifier param in post signature:\n%s", out)
	}
	if !strings.Contains(out, "def put(self, request: BaseRequest, identifier: str):") {
		t.Fatalf("expected identifier param in put signature:\n%s", out)
	}
}

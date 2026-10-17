package identity

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestRejectsTrailingJSONValues(t *testing.T) {
    cases := []struct { payload string; valid bool } {
        {`{"value":"yes"}`,true},
        {"  {\"value\":\"yes\"}  ",true},
        {`{"value":"yes"}{"value":"no"}`,false},
        {`{"value":"yes"} true`,false},
        {`{"value":"yes"} trailing`,false},
        {`{"unexpected":"secret"}`,false},
    }
    for _,c := range cases {
        req := httptest.NewRequest(http.MethodPost,"/strict-json",
            strings.NewReader(c.payload))
        response := httptest.NewRecorder()
        var value struct { Value string `json:"value"` }
        err := decodeJSON(response,req,&value)
        if (err==nil) != c.valid { t.Errorf("payload=%q valid=%v err=%v",c.payload,c.valid,err) }
    }
}

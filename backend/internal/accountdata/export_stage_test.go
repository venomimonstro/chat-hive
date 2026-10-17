package accountdata

import (
    "context"
    "errors"
    "io"
    "os"
    "testing"
)

type stubExportStore struct { fail bool; body string }
func (s stubExportStore) WriteExport(_ context.Context,_ string,w io.Writer)error{
    if _,err:=io.WriteString(w,s.body);err!=nil{return err}
    if s.fail{return errors.New("database became unavailable")}
    return nil
}
func (s stubExportStore) DeleteAccount(context.Context,string)error{return nil}

func TestAccountExportStagingDoesNotExposePartialData(t *testing.T){
    file,_,err:=stageAccountExport(context.Background(),
        stubExportStore{fail:true,body:`{"private":"partial"`},"user")
    if err==nil || file!=nil {t.Fatalf("failed export was promoted: %v",err)}
}
func TestAccountExportStagingUsesPrivateFileAndFullLength(t *testing.T){
    body:=`{"private":"complete"}`
    file,size,err:=stageAccountExport(context.Background(),stubExportStore{body:body},"user")
    if err!=nil {t.Fatal(err)}
    defer os.Remove(file.Name());defer file.Close()
    info,err:=file.Stat()
    if err!=nil {t.Fatal(err)}
    if info.Mode().Perm()!=0600 {t.Fatalf("insecure temporary file mode: %v",info.Mode())}
    if size!=int64(len(body)){t.Fatalf("wrong staged file size: %d",size)}
    data,err:=io.ReadAll(file)
    if err!=nil || string(data)!=body {t.Fatalf("staged export differs: %v",err)}
}

package onboarding
import ("context";"errors";"testing")

type coreFakeStore struct{}
func (*coreFakeStore) ListInterests(context.Context)([]Interest,error){return nil,nil}
func (*coreFakeStore) GetProfile(context.Context,string)(Profile,error){return Profile{},nil}
func (*coreFakeStore) Complete(_ context.Context,input CompleteInput)(Profile,error){
 return Profile{UserID:input.UserID,Username:input.Username,DisplayName:input.DisplayName,Completed:true},nil
}
func TestCoreOnboardingAllowsNoInterests(t *testing.T) {
 service:=NewService(&coreFakeStore{}); service.SetCoreOnly(true)
 _,err:=service.Complete(context.Background(),CompleteInput{UserID:"id",Username:"new_user",DisplayName:"Tester"})
 if err!=nil {t.Fatal(err)}
}
func TestFullOnboardingStillRequiresInterests(t *testing.T) {
 service:=NewService(&coreFakeStore{})
 _,err:=service.Complete(context.Background(),CompleteInput{UserID:"id",Username:"new_user",DisplayName:"Tester"})
 if !errors.Is(err,ErrInvalidProfile) {t.Fatalf("expected interest validation, got %v",err)}
}

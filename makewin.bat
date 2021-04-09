rem get version
git describe --tag > temp.txt
set /p VERSION=<temp.txt

rem get commit hash
git log -1 --format=%%H > temp.txt
set /p COMMIT=<temp.txt

rem clear
del temp.txt


set LDFLAG="-X github.com/cosmos/cosmos-sdk/version.Name=crypto-org-chain-chain -X github.com/cosmos/cosmos-sdk/version.ServerName=chain-maind -X github.com/cosmos/cosmos-sdk/version.Version=%VERSION% -X github.com/cosmos/cosmos-sdk/version.Commit=%COMMIT%"
go install -mod=readonly -ldflags %LDFLAG% -tags cgo,ledger,!test_ledger_mock,!ledger_mock,!ledger_zemu ./cmd/chain-maind
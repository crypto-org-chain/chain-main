# Vesting configuration
Vesting can be configured at genesis time.
There are different "vesting account types".
The intended one is "delayed vesting" where the full amount is locked until a specified date.
It can be configured using command-line tooling (the time is currently specified using RFC-3339 format
`2006-01-02T15:04:05Z` (for UTC) and `2006-01-02T15:04:05+07:00` (for other timezones)),
e.g.:

```
chain-maind add-genesis-account cro18mdlqc9w2ecdveky9sqz9yum4yze0ec2wny5sx 20000000000cro --vesting-amount 20000000000cro --vesting-end-time 2006-01-02T15:04:05+07:00
```

The first amount specified the total amount, the second one (after `--vesting-amount` flag) 
specifies the amount to be locked (i.e. it needs to be less or equal to the total amount).

Note that if `--vesting-start-time` flag is provided, a different vesting account type is used 
("continuous vesting"). In this vesting type, the amount is continuously unlocked which may not be intended.
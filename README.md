# Note:  
Must run in this sequence to make node work correctly:
1. Start Validator1
2. Start Node1
3. Start validator2
3. Start Node2

# Build validatos
`./buildValidators.sh`

# Build proto  
`protoc --go_out=. ./proto/*.proto`  

# A Key pair
```
Seckey: 4aac41b5cb665b93e031faa751944b1f14d77cb17322403cba8df1d6e4541a4d
Pubkey: 841c5235ec7f4eed02b3f3bb60622d3ed0aba74016f4850c6d7c962656a4b78d72a15caeef62dfe656d03990590c0026
Address: 0x5e65ce8502fb2a85f061b3fd8256d61cc8c9d440
```

# TODO
- sync data before run POH
- update base on greedy transaction fee
- validate signs in VoteResult
- better manage connections (CLI, v.v)
- update data storage, save snap shot
    POH:
        LastBlock    
    AccountState:
        [{Address: lasthash}, ...]



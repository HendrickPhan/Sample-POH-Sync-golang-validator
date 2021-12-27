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



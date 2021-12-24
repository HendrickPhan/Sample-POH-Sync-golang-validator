~~1. Go routine for hash process~~  
~~2. Add transaction during hash~~  
~~3. udp server for validator for testing~~  
4. validate data from validator  
5. sync data from valdator  
6. forward transaction to leader  
~~7. receive block from leader~~   
~~8. save branches of POH~~  
~~9. handle forks from POH~~  
~~10. valid range of count to start, stop hashing~~  
~~11.protobuf~~

# POH optimize
hash per second
implement tick per second
tick per slot 
Add throttle hash by tick 
e.x:
"hash_per_second": 1600,
"tick_per_second": 160,
"tick_per_slot": 64
With these config we have this:
hash per tick = 1600 / 160 = 10
tick duration = 1000 /160 = 6.25 ms
=> 10 hash every 6.25 ms 

when validator receive transaction it will forward it to next leader

when validator receive tick from leader it will send to miner to check


TODO: send tick to child
TODO: receive tick validate result from child
TODO: update data storage, save snap shot
    
    - snap shot datastruct:
    POH:
        LastBlock    
    AccountState:
        [{Address: lasthash}, ...]

TODO: broadcast confrim block to child
TODO: send sync data to child


TODO:  validate data in HandleVoteResult
TODO: validate POH  
TODO: sync data before run POH

TODO: update base on greedy transaction fee
TODO: handle case no block were valid when receive vote leader??

Flow of validator

~~when a node start it will send all other node to mark started~~  
~~when all node has been start then it will start do poh~~
 

# Build proto  
`protoc --go_out=. ./proto/*.proto`  


nhận được tick từ leader mới trước khi nhận được voted result từ leader cũ
?? how to solve this
wait a bit for orther node receive voted result befor send tick

add tick to next leader tick chan, and
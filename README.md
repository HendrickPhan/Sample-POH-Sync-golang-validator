~~1. Go routine for hash process~~  
~~2. Add transaction during hash~~  
~~3. udp server for validator for testing~~  
4. validate data from validator  
5. sync data from valdator  
6. forward transaction to leader  
7. receive block from leader   
~~8. save branches of POH~~  
~~9. handle forks from POH~~  
~~10. valid range of count to start, stop hashing~~  


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


TODO: take transaction from recorder
TODO:  validate data in HandleVoteResult
TODO: validate POH  
TODO: sync data before run POH

TODO: update base on greedy transaction fee
TODO: handle case no block were valid when receive vote leader??


Flow of validator

~~when a node start it will send all other node to mark started~~  
~~when all node has been start then it will start do poh~~

start with leader
leader will create tick and send tick to orther validator
orther validator will save tick
at tick 62 orther validator will send voted block to leader, leader will choose most voted block  

leader

0 => 61 generate tick and broad cast to validators
62 => receive votes and generate block, broadcast block to validators
63 => 
# archie
A distributed scheduling protocol.

--------------------
Run a server:
--------------------

go run archie.go -b 127.0.0.1:7777 peersfile.txt

First arg is -b or -j, depending on a bootstrap run or a joining run
Second arg is the address of the node. It must be located in peersfile.
Third arg is the peersfile.txt that refers to all other nodes.

---------------
Run a client:
---------------

go run ui/client.go 0 peersfile.txt

Similar as above, but specify the node number. The client automatically runs on the port of its server + 1.

---------------
Peersfile.txt
----------------
NodeNumber,Name,Address

i.e. 
0,Ivan,127.0.0.1:7777

ALL peers must be up and running for Archie to start.

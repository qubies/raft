# README #
Raft can be thought of as a distributed state machine manager, controlling state changes so that all servers agree on the status of the raft. This program is a testbed for a simplified version of the RAFT algorithm. It is not intended to be used in production, rather it is a simple demonstration of the RAFT algorithm, developed by an undergraduate CS student.

# INSTALLATION on Ubuntu #
Tested using go 1.81, and NCurses as dependancies.
To install Ncurses on Ubuntu:
```
#!bash
sudo apt-get install libncurses5-dev
```
### Single member INSTALLATION  ###
Clone the repository:
```
#!bash
git clone https://github.com/qubies/raft 

```
From within the /raft folder, run go run raft.go -i [address:port to connect to - localhost if none yet]
State files are stored within /tmp
### NOTE ###
You cannot run multiple instances of raft from the same computer without Docker. On load the program will load the persist.raft file which will contain the same ID as the other running instance, and cause a conflict.

### MULTIPLE INSTANCE INSTALLATION WITH DOCKER ###
```
#!bash
git clone https://bitbucket.org/qubies/raft.git && docker build --rm --no-cache -t raft:latest raft/dockerfile

```
This creates a new docker image called raft. You can work with the image by running:

```
#!bash

### omit the name argument to generate multiple containers.
docker run -ti --name raft raft bash
```

To run a disposable, random raft run
```
#!bash

docker run --rm -ti raft bash
```
This builds a new raft container, with a random name.

You can attach and detach from the process using:
```
#!bash

docker attach [container name]
```
To detach from the container without stopping use ^p^q

#### When attached to a container, begin a raft through: ####
```
#!bash

go run raft.go -i localhost
```
to attach to a running raft, point to any one of the members with the -i argument:
```
#!bash

go run raft.go -i 172.17.0.2:33333
```
where the *IPaddress:Port* combination is the address indicated in the status of a running raft member.



### What does RAFT stand for? ###

* Redundant and fault tolerant
* Let the chaos monkey loose!
* A raft is built with logs (event logs!)
* You can use a raft to get off the island of Paxos... whee!

### Testing ###

* Modules are being constructed independently. Each module has a test file with it.
* Tests are automatically run during the docker installation method, through the dockerfile.
* To manually run module tests, switch to the module directory and run

```
#!Go

go test
```

* To run all tests, from the parent directory of raft run
```
#!Go
go test ./...
```
* To run all tests with race detection, from the parent directory of raft run
```
#!Go
go test -race ./...
```

This project is built by Tobias Renwick with the Supervision of Dr. Cameron MacDonell of MacEwan University
This project is based on the RAFT distributed consensus algorithm as defined @ https://raft.github.io. Thank you for the wonderful resources!

<p align="center">
  <img width="320" src="https://user-images.githubusercontent.com/32441764/145425943-5f389366-a4c1-4ecc-9c3f-cecb79b4e0e6.png">
</p>

# Statera
Statera is a L7 Load Balancer for the HTTP protocol.

**Authors:** Matheus H. Freitas and Vitor B. C. Souza 

_________________

## Paper (portuguese)

[Statera: Um balanceador de carga rápido e flexível paraaplicações HTTP na nuvem](https://sol.sbc.org.br/index.php/wscad/article/view/21948/21771)

## Requirements to build from source
Go ^1.18 or Docker 
(**Docker recommended**)

## Commands for Docker
After clone the repository:
### Build and run
This command will compile Statera from the source and run docker-compose up to execute it.
```
$ make build-run
```

### Build
It will compile from the source and generate a tagged, production-ready image.
```
$ make
```

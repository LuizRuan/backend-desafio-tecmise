# Setup do Banco de Dados e Backend - TecMise

Este guia irá te ajudar a configurar **o banco de dados PostgreSQL** e **o backend em Go** para rodar o sistema TecMise em ambiente local.

## 1. Instalar o PostgreSQL

Você precisa do **PostgreSQL** instalado em sua máquina.

- **Windows:**  
  Baixe o instalador oficial:  
  [https://www.postgresql.org/download/windows/](https://www.postgresql.org/download/windows/)

- **Linux (Ubuntu):**
  ```bash
  sudo apt update
  sudo apt install postgresql postgresql-contrib
Mac (Homebrew):

bash
Copiar
Editar
brew install postgresql
Dica: Após instalar, abra o pgAdmin ou acesse o psql pelo terminal para gerenciar o banco.

2. Criar o Banco de Dados
Acesse o PostgreSQL via pgAdmin ou pelo terminal:

bash
Copiar
Editar
psql -U postgres
Crie um banco chamado clientes_db (pode mudar o nome, mas lembre de alterar na string de conexão):

sql
Copiar
Editar
CREATE DATABASE clientes_db;
No pgAdmin ou psql, selecione o banco e execute o script schema.sql fornecido no projeto para criar as tabelas necessárias.

3. Instalar o Go
Baixe e instale o Go pela página oficial:
https://go.dev/dl/

Dica: Após instalar, teste no terminal:

bash
Copiar
Editar
go version
4. Configurar a Conexão com o Banco
O projeto usa um arquivo db.go para conexão.
Exemplo de string de conexão no Go:

go
Copiar
Editar
// db.go
connStr := "host=localhost port=5432 user=postgres password=SENHA_DO_SEU_POSTGRES dbname=clientes_db sslmode=disable"
Altere SENHA_DO_SEU_POSTGRES para a sua senha do postgres.

Se o usuário não for postgres, troque para o seu usuário.

Dica:
Você pode usar variáveis de ambiente ou um .env (opcional) para não deixar senha exposta.

5. Instalar as Dependências do Backend
Na pasta backend, rode:

bash
Copiar
Editar
go mod tidy
Se necessário, rode também:

bash
Copiar
Editar
go get github.com/lib/pq
go get golang.org/x/crypto/bcrypt
6. Rodar o Servidor Backend
Ainda na pasta backend:

bash
Copiar
Editar
go run main.go db.go
O backend ficará disponível em http://localhost:8080

7. Testar a Conexão
Acesse no navegador ou use um software como Postman para testar:

Cadastro:
POST http://localhost:8080/register

Login:
POST http://localhost:8080/login

Criar Estudante:
POST http://localhost:8080/estudantes

Problemas Comuns
Erro de conexão: Verifique host, usuário, senha e nome do banco.

Porta em uso: Certifique-se que o PostgreSQL está na porta 5432 (padrão).

Permissão negada: Confira permissões do usuário no banco.

Tabela não existe: Execute o schema.sql para criar todas as tabelas antes de rodar o backend.

Observação
Este README cobre apenas banco de dados e backend.
O frontend deve ser configurado na pasta correspondente (veja o README da raiz).

Pronto! Agora o backend está rodando e conectado ao banco PostgreSQL.
Se tiver dúvidas, procure a equipe TecMise!
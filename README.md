# TecMise Backend — API de Gestão Escolar

Bem-vindo ao **TecMise Backend**, o núcleo de processamento e integração de dados do sistema completo de gestão escolar **TecMise**.

---

## 📚 Sobre o Projeto

Este backend foi desenvolvido com foco em segurança, produtividade e escalabilidade, utilizando **Go (Golang)**, **PostgreSQL 16.9** e **sqlc**. Ele oferece uma API RESTful robusta para cadastro, autenticação e gerenciamento de estudantes, usuários, anos e turmas, sempre vinculando cada registro ao usuário logado.

---

## ⚙️ Tecnologias Utilizadas

- **Go (Golang):** v1.24.5  
- **PostgreSQL:** v16.9  
- **sqlc:** v1.29.0  
- **pgAdmin:** Recomendado para administração do banco  
- **Pacotes Go:**  
  - `github.com/lib/pq` (driver Postgres)
  - `github.com/joho/godotenv` (variáveis de ambiente)
  - `golang.org/x/crypto` (hash de senha e segurança)

---

## 📂 Estrutura do Projeto

/
├── backend/ # Código fonte do backend Go
│ ├── handler/ # Handlers das rotas (CRUD, login, etc.)
│ ├── model/ # Structs das entidades e tipos
│ ├── main.go # Ponto de entrada do servidor
│ ├── go.mod # Dependências Go
│ └── .env.example # Exemplo de configuração
├── frontend/ # Frontend (veja README da pasta)
└── schema.sql # Script SQL para criar as tabelas do banco

markdown
Copiar
Editar

---

## 🚀 **Instalação Rápida**

### 1. **Pré-requisitos**

- [Go](https://golang.org/doc/install) v1.24.5 ou superior  
- [PostgreSQL](https://www.postgresql.org/download/) v16.9  
- [sqlc](https://docs.sqlc.dev/en/latest/overview/install.html) v1.29.0  
- [pgAdmin](https://www.pgadmin.org/) (opcional, interface gráfica para o banco)
- [Git](https://git-scm.com/)

### 2. **Clone o Projeto**

```bash
git clone https://github.com/seuusuario/tecmise.git
cd tecmise/backend
3. Configuração do Banco de Dados
Crie um banco de dados chamado clientes_db no PostgreSQL.

Execute o script schema.sql para criar as tabelas (anos, estudantes, usuarios).

Exemplo de conexão no pgAdmin:

Host: localhost

Usuário: postgres

Senha: sua senha

4. Configuração do Ambiente
Copie o arquivo .env.example para .env e edite a variável DATABASE_URL conforme sua instalação:

bash
Copiar
Editar
DATABASE_URL=postgres://usuario:senha@localhost:5432/clientes_db?sslmode=disable
5. Instale as Dependências
bash
Copiar
Editar
go mod tidy
6. Rode o Servidor Backend
bash
Copiar
Editar
go run .
O backend ficará disponível em http://localhost:8080

📋 Tabelas e Relacionamentos
Tabela anos
Campo	Tipo	Descrição
id	int	PK, autoincrement
nome	varchar(120)	Nome do ano/turma

Tabela estudantes
Campo	Tipo	Descrição
id	int	PK, autoincrement
nome	varchar(120)	Nome completo
cpf	varchar(14)	CPF do estudante
email	varchar(200)	E-mail
data_nascimento	date	Data de nascimento
telefone	varchar(32)	Telefone
foto_url	text	Foto de perfil (Base64 ou URL)
ano_id	int	FK para anos
turma_id	int	(não utilizada se for só ano_id)
usuario_id	int	FK para usuarios

Tabela usuarios
Campo	Tipo	Descrição
id	int	PK, autoincrement
nome	varchar(100)	Nome do usuário
email	varchar(200)	E-mail (único)
senha_hash	varchar(300)	Senha criptografada
foto_url	text	Foto de perfil

Observações:

Cada usuário só enxerga e gerencia seus próprios estudantes e anos.

Exclusão em cascata: ao remover um ano/turma, os estudantes vinculados também são apagados.

🛠️ Principais Rotas da API
Método	Rota	Descrição
POST	/register	Cadastro de usuário
POST	/login	Login de usuário
GET	/api/estudantes	Lista estudantes do usuário
POST	/api/estudantes	Cria estudante
PUT	/api/estudantes/:id	Edita estudante
DELETE	/api/estudantes/:id	Remove estudante
GET	/api/anos	Lista anos/turmas do usuário
POST	/api/anos	Cria novo ano/turma
DELETE	/api/anos/:id	Remove ano/turma
GET	/api/usuario	Busca perfil do usuário
PUT	/api/perfil	Edita perfil do usuário

Autenticação:
O email do usuário logado é passado no header X-User-Email para vinculação dos registros.

🏆 Diferenciais Técnicos
Segurança: Senhas com hash bcrypt, validação de CPF e email, CORS ajustável.

Performance: Queries otimizadas e respostas rápidas mesmo com grande volume de dados.

Arquitetura Limpa: Separação clara entre handlers, models e rotas.

Pronto para Deploy: Variáveis de ambiente, scripts limpos, código comentado.

🐞 Erros Comuns e Soluções
Erro 431 ou 500: Verifique permissões de firewall/antivírus e se o backend está rodando como admin.

Banco não conecta: Cheque a string de conexão no .env e se o Postgres está rodando.

API responde "Usuário não autenticado": Veja se o header X-User-Email está presente e correto.

Duplicidade de CPF: Cada estudante deve ter CPF único por usuário.

💬 Contato e Contribuições
Dúvidas, sugestões ou pull requests são bem-vindos!
Entre em contato pelo GitHub ou via issues.

TecMise — Sua escola mais conectada, moderna e eficiente!
Desenvolvido com 💙 por profissionais que amam código limpo.

yaml
Copiar
Editar

---

Se precisar de mais alguma seção específica (ex: **Deploy em produção**, **Testes**, **Dicas de organização**), só pedir!  
Se quiser personalizar o nome do usuário do GitHub, me avise.

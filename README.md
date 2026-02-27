# GoBase 🚀

GoBase é um framework BaaS (Backend-as-a-Service) moderno, leve e extensível, inspirado no Supabase, mas focado na simplicidade do ecossistema Go. Ele oferece um motor de API dinâmico, gerenciamento de schema automático e segurança em tempo real.

---

## ✨ Principais Diferenciais

### 1. Motor de API 100% Dinâmico
- **CRUD Automático**: Simplesmente crie uma tabela e a API REST já estará disponível.
- **Expansão de Relacionamentos**: Use `?select=*,author(*)` para carregar objetos aninhados automaticamente (One-to-Many e Many-to-One).
- **Inserções Recursivas**: Envie objetos aninhados em um `POST` e o GoBase cria os registros pai/filho na ordem correta dentro de uma única transação.

### 2. Segurança e Multi-Tenancy
- **RLS (Row Level Security)**: Isolamento total de dados entre clientes (tenants) direto no nível do banco de dados.
- **RBAC Nativo**: Controle de acesso baseado em cargos e permissões.
- **SQL Sanitization**: Motor de consulta blindado com `pgx.Identifier` para evitar SQL Injection em colunas dinâmicas.

### 3. Experiência do Desenvolvedor (DX)
- **API Console Interativo**: Um Swagger embutido no painel para testar endpoints com métricas de performance em tempo real.
- **Realtime Broadcast**: Notificações instantâneas via WebSockets (Hub centralizado) para qualquer mudança de dados ou schema.
- **Trilha de Auditoria**: Log completo de quem alterou o quê, quando e onde.

---

## 🛠️ Como Começar

### Pré-requisitos
- [Go](https://golang.org/dl/) (v1.23+)
- [Docker](https://www.docker.com/get-started) e Docker Compose
- [Node.js](https://nodejs.org/) (Para desenvolvimento do Admin UI)

### Guia Rápido
1. **Subir o Banco de Dados:**
   ```bash
   make up
   ```
2. **Rodar a Aplicação (Full Stack):**
   ```bash
   # Compila frontend e roda o servidor Go
   make all
   make run
   ```
3. **Acessar o Painel Admin:**
   Abra [http://localhost:8080/admin](http://localhost:8080/admin) no seu navegador.

---

## 🛡️ Validação de Dados (BaaS Core)

Configure regras de validação direto no ícone de "Escudo" na aba Schema:

| Regra | Descrição | Exemplo |
|---|---|---|
| `min` / `max` | Limites numéricos ou de caracteres | `{ "min": 5 }` |
| `pattern` | Validação via RegEx | `{ "pattern": "^[A-Z]+$" }` |
| `email` | Formato de e-mail válido | `{ "email": true }` |
| `json_schema` | Validação complexa para campos JSONB | [Veja no código](internal/validator/validator.go) |

---

## 📁 Estrutura Técnica

- `/internal/api`: Handlers REST dinâmicos e filtros estilo PostgREST.
- `/internal/realtime`: Gerenciamento de WebSockets e Broadcasting.
- `/internal/validator`: Motor de validação de regras em Go.
- `/admin-ui`: Interface em React (Vite) com Design System Premium (OLED Dark Mode).

---

Desenvolvido com ❤️ usando Go, PostgreSQL e React.

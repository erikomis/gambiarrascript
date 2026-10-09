CREATE TABLE tarefas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    usuario_id INTEGER NOT NULL REFERENCES usuarios (id),
    titulo TEXT NOT NULL,
    feita INTEGER NOT NULL DEFAULT 0,
    criada_em TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX tarefas_por_usuario ON tarefas (usuario_id);

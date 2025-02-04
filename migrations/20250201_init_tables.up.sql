-- CREATE TABLE IF NOT EXISTS example ();
CREATE TABLE IF NOT EXISTS graphs (
    id INTEGER PRIMARY KEY,
    max_node_id INTEGER,
    max_edge_id INTEGER,
    name TEXT
);

CREATE TABLE IF NOT EXISTS services (
    id INTEGER PRIMARY KEY,
    graph_id INTEGER REFERENCES graphs(id) ON DELETE CASCADE,
    name TEXT,
    description TEXT,
    x REAL,
    y REAL
);

CREATE TABLE IF NOT EXISTS relations (
    id INTEGER PRIMARY KEY,
    graph_id INTEGER REFERENCES graphs(id) ON DELETE CASCADE,
    name TEXT,
    description TEXT,
    from_service INTEGER REFERENCES services(id) ON DELETE CASCADE,
    to_service INTEGER REFERENCES services(id) ON DELETE CASCADE
);

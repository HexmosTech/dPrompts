CREATE TABLE dprompts_results (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job_id BIGINT UNIQUE,
    response JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    group_id INT,
    CONSTRAINT fk_group
        FOREIGN KEY (group_id)
        REFERENCES dprompt_groups(id)
);

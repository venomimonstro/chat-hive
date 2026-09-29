ALTER TABLE profiles
    ADD COLUMN onboarding_completed_at TIMESTAMPTZ;

CREATE TABLE interests (
    slug TEXT PRIMARY KEY,
    label_ru TEXT NOT NULL,
    label_en TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE user_interests (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    interest_slug TEXT NOT NULL REFERENCES interests(slug) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, interest_slug)
);

CREATE INDEX user_interests_interest_idx ON user_interests (interest_slug, user_id);

INSERT INTO interests (slug, label_ru, label_en, sort_order) VALUES
    ('music', 'Музыка', 'Music', 10),
    ('games', 'Игры', 'Games', 20),
    ('movies', 'Кино', 'Movies', 30),
    ('technology', 'Технологии', 'Technology', 40),
    ('photo', 'Фото', 'Photography', 50),
    ('travel', 'Путешествия', 'Travel', 60),
    ('sport', 'Спорт', 'Sport', 70),
    ('fashion', 'Мода', 'Fashion', 80),
    ('design', 'Дизайн', 'Design', 90),
    ('cars', 'Автомобили', 'Cars', 100),
    ('business', 'Бизнес', 'Business', 110),
    ('books', 'Книги', 'Books', 120);

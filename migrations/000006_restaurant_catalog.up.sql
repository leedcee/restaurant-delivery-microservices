ALTER TABLE restaurants
    ADD COLUMN cuisine text NOT NULL DEFAULT '',
    ADD COLUMN eta text NOT NULL DEFAULT '',
    ADD COLUMN rating numeric(2,1) NOT NULL DEFAULT 0 CHECK (rating BETWEEN 0 AND 5),
    ADD COLUMN art text NOT NULL DEFAULT 'bread' CHECK (art IN ('bread', 'pasta', 'sushi'));

UPDATE restaurants
SET cuisine = 'Европейская · Завтраки',
    eta = '25–35 минут',
    rating = 4.8,
    art = 'bread'
WHERE id = '11111111-1111-1111-1111-111111111111';

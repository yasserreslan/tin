-- orders' tables and the orders record.sh pays: two that pay, and two for each bug. Run again to start over.
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS orders;
CREATE TABLE orders (id bigint PRIMARY KEY, customer text NOT NULL, amount_cents bigint NOT NULL, currency text NOT NULL, status text NOT NULL DEFAULT 'unpaid');
CREATE TABLE payments (order_id bigint PRIMARY KEY REFERENCES orders(id), usd_cents bigint NOT NULL, rate text NOT NULL, paid_at timestamptz NOT NULL DEFAULT now());
INSERT INTO orders (id, customer, amount_cents, currency) VALUES
	(101, 'Lina', 4999, 'EUR'),   -- paid (the rate from the website)
	(102, 'Omar', 12000, 'EUR'),  -- paid
	(201, 'Mateo', 250000, 'ARS'),-- bug 1: a currency the website does not price
	(202, 'Sofia', 99000, 'ARS'),
	(301, 'Rami', 7500, 'GBP'),   -- bug 2: two pay clicks at once
	(302, 'Noor', 3200, 'GBP'),
	(401, 'Jad', 0, 'EUR'),       -- bug 3: a free order
	(402, 'Tala', 0, 'GBP');

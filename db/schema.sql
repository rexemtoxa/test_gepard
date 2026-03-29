--
-- PostgreSQL database dump
--

\restrict lmOM4njEk0QW4NM54zciqORez56JvImdfgLGkB0sY2xqJysCFikGOKSrvvCNql6

-- Dumped from database version 17.9
-- Dumped by pg_dump version 17.9

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: ledger_entries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ledger_entries (
    id bigint NOT NULL,
    request_tx_id text,
    reverses_entry_id bigint,
    entry_type text NOT NULL,
    signed_amount numeric NOT NULL,
    prev_entry_id bigint,
    balance_after numeric NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ledger_entries_balance_after_check CHECK ((balance_after >= (0)::numeric)),
    CONSTRAINT ledger_entries_check CHECK ((((entry_type = 'apply'::text) AND (request_tx_id IS NOT NULL) AND (reverses_entry_id IS NULL)) OR ((entry_type = 'cancel'::text) AND (request_tx_id IS NULL) AND (reverses_entry_id IS NOT NULL)))),
    CONSTRAINT ledger_entries_entry_type_check CHECK ((entry_type = ANY (ARRAY['apply'::text, 'cancel'::text])))
);


--
-- Name: ledger_entries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.ledger_entries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: ledger_entries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.ledger_entries_id_seq OWNED BY public.ledger_entries.id;


--
-- Name: operation_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.operation_requests (
    tx_id text NOT NULL,
    source text NOT NULL,
    state text NOT NULL,
    amount numeric NOT NULL,
    result_status text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT operation_requests_amount_check CHECK ((amount > (0)::numeric)),
    CONSTRAINT operation_requests_result_status_check CHECK ((result_status = ANY (ARRAY['applied'::text, 'rejected_insufficient_funds'::text]))),
    CONSTRAINT operation_requests_source_check CHECK ((source = ANY (ARRAY['game'::text, 'payment'::text, 'service'::text]))),
    CONSTRAINT operation_requests_state_check CHECK ((state = ANY (ARRAY['deposit'::text, 'withdraw'::text])))
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version character varying NOT NULL
);


--
-- Name: ledger_entries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries ALTER COLUMN id SET DEFAULT nextval('public.ledger_entries_id_seq'::regclass);


--
-- Name: ledger_entries ledger_entries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_pkey PRIMARY KEY (id);


--
-- Name: ledger_entries ledger_entries_prev_entry_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_prev_entry_id_key UNIQUE (prev_entry_id);


--
-- Name: ledger_entries ledger_entries_request_tx_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_request_tx_id_key UNIQUE (request_tx_id);


--
-- Name: ledger_entries ledger_entries_reverses_entry_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_reverses_entry_id_key UNIQUE (reverses_entry_id);


--
-- Name: operation_requests operation_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.operation_requests
    ADD CONSTRAINT operation_requests_pkey PRIMARY KEY (tx_id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: ledger_entries_apply_odd_latest_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ledger_entries_apply_odd_latest_idx ON public.ledger_entries USING btree (id DESC) INCLUDE (signed_amount) WHERE ((entry_type = 'apply'::text) AND ((id % (2)::bigint) = 1));


--
-- Name: ledger_entries ledger_entries_prev_entry_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_prev_entry_id_fkey FOREIGN KEY (prev_entry_id) REFERENCES public.ledger_entries(id);


--
-- Name: ledger_entries ledger_entries_request_tx_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_request_tx_id_fkey FOREIGN KEY (request_tx_id) REFERENCES public.operation_requests(tx_id);


--
-- Name: ledger_entries ledger_entries_reverses_entry_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledger_entries
    ADD CONSTRAINT ledger_entries_reverses_entry_id_fkey FOREIGN KEY (reverses_entry_id) REFERENCES public.ledger_entries(id);


--
-- PostgreSQL database dump complete
--

\unrestrict lmOM4njEk0QW4NM54zciqORez56JvImdfgLGkB0sY2xqJysCFikGOKSrvvCNql6


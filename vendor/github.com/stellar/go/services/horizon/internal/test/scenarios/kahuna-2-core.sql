running recipe
recipe finished, closing ledger
ledger closed
--
-- PostgreSQL database dump
--

-- Dumped from database version 10.4
-- Dumped by pg_dump version 10.4

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET client_min_messages = warning;
SET row_security = off;

DROP INDEX IF EXISTS public.signersaccount;
DROP INDEX IF EXISTS public.sellingissuerindex;
DROP INDEX IF EXISTS public.scpquorumsbyseq;
DROP INDEX IF EXISTS public.scpenvsbyseq;
DROP INDEX IF EXISTS public.priceindex;
DROP INDEX IF EXISTS public.ledgersbyseq;
DROP INDEX IF EXISTS public.histfeebyseq;
DROP INDEX IF EXISTS public.histbyseq;
DROP INDEX IF EXISTS public.buyingissuerindex;
DROP INDEX IF EXISTS public.accountbalances;
ALTER TABLE IF EXISTS ONLY public.txhistory DROP CONSTRAINT IF EXISTS txhistory_pkey;
ALTER TABLE IF EXISTS ONLY public.txfeehistory DROP CONSTRAINT IF EXISTS txfeehistory_pkey;
ALTER TABLE IF EXISTS ONLY public.trustlines DROP CONSTRAINT IF EXISTS trustlines_pkey;
ALTER TABLE IF EXISTS ONLY public.storestate DROP CONSTRAINT IF EXISTS storestate_pkey;
ALTER TABLE IF EXISTS ONLY public.signers DROP CONSTRAINT IF EXISTS signers_pkey;
ALTER TABLE IF EXISTS ONLY public.scpquorums DROP CONSTRAINT IF EXISTS scpquorums_pkey;
ALTER TABLE IF EXISTS ONLY public.pubsub DROP CONSTRAINT IF EXISTS pubsub_pkey;
ALTER TABLE IF EXISTS ONLY public.publishqueue DROP CONSTRAINT IF EXISTS publishqueue_pkey;
ALTER TABLE IF EXISTS ONLY public.peers DROP CONSTRAINT IF EXISTS peers_pkey;
ALTER TABLE IF EXISTS ONLY public.offers DROP CONSTRAINT IF EXISTS offers_pkey;
ALTER TABLE IF EXISTS ONLY public.ledgerheaders DROP CONSTRAINT IF EXISTS ledgerheaders_pkey;
ALTER TABLE IF EXISTS ONLY public.ledgerheaders DROP CONSTRAINT IF EXISTS ledgerheaders_ledgerseq_key;
ALTER TABLE IF EXISTS ONLY public.ban DROP CONSTRAINT IF EXISTS ban_pkey;
ALTER TABLE IF EXISTS ONLY public.accounts DROP CONSTRAINT IF EXISTS accounts_pkey;
ALTER TABLE IF EXISTS ONLY public.accountdata DROP CONSTRAINT IF EXISTS accountdata_pkey;
DROP TABLE IF EXISTS public.txhistory;
DROP TABLE IF EXISTS public.txfeehistory;
DROP TABLE IF EXISTS public.trustlines;
DROP TABLE IF EXISTS public.storestate;
DROP TABLE IF EXISTS public.signers;
DROP TABLE IF EXISTS public.scpquorums;
DROP TABLE IF EXISTS public.scphistory;
DROP TABLE IF EXISTS public.pubsub;
DROP TABLE IF EXISTS public.publishqueue;
DROP TABLE IF EXISTS public.peers;
DROP TABLE IF EXISTS public.offers;
DROP TABLE IF EXISTS public.ledgerheaders;
DROP TABLE IF EXISTS public.ban;
DROP TABLE IF EXISTS public.accounts;
DROP TABLE IF EXISTS public.accountdata;
DROP EXTENSION IF EXISTS plpgsql;
DROP SCHEMA IF EXISTS public;
--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA public;


--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA public IS 'standard public schema';


--
-- Name: plpgsql; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS plpgsql WITH SCHEMA pg_catalog;


--
-- Name: EXTENSION plpgsql; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION plpgsql IS 'PL/pgSQL procedural language';


SET default_tablespace = '';

SET default_with_oids = false;

--
-- Name: accountdata; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.accountdata (
    accountid character varying(56) NOT NULL,
    dataname character varying(64) NOT NULL,
    datavalue character varying(112) NOT NULL,
    lastmodified integer DEFAULT 0 NOT NULL
);


--
-- Name: accounts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.accounts (
    accountid character varying(56) NOT NULL,
    balance bigint NOT NULL,
    seqnum bigint NOT NULL,
    numsubentries integer NOT NULL,
    inflationdest character varying(56),
    homedomain character varying(32) NOT NULL,
    thresholds text NOT NULL,
    flags integer NOT NULL,
    lastmodified integer NOT NULL,
    CONSTRAINT accounts_balance_check CHECK ((balance >= 0)),
    CONSTRAINT accounts_numsubentries_check CHECK ((numsubentries >= 0))
);


--
-- Name: ban; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ban (
    nodeid character(56) NOT NULL
);


--
-- Name: ledgerheaders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ledgerheaders (
    ledgerhash character(64) NOT NULL,
    prevhash character(64) NOT NULL,
    bucketlisthash character(64) NOT NULL,
    ledgerseq integer,
    closetime bigint NOT NULL,
    data text NOT NULL,
    CONSTRAINT ledgerheaders_closetime_check CHECK ((closetime >= 0)),
    CONSTRAINT ledgerheaders_ledgerseq_check CHECK ((ledgerseq >= 0))
);


--
-- Name: offers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.offers (
    sellerid character varying(56) NOT NULL,
    offerid bigint NOT NULL,
    sellingassettype integer NOT NULL,
    sellingassetcode character varying(12),
    sellingissuer character varying(56),
    buyingassettype integer NOT NULL,
    buyingassetcode character varying(12),
    buyingissuer character varying(56),
    amount bigint NOT NULL,
    pricen integer NOT NULL,
    priced integer NOT NULL,
    price double precision NOT NULL,
    flags integer NOT NULL,
    lastmodified integer NOT NULL,
    CONSTRAINT offers_amount_check CHECK ((amount >= 0)),
    CONSTRAINT offers_offerid_check CHECK ((offerid >= 0))
);


--
-- Name: peers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.peers (
    ip character varying(15) NOT NULL,
    port integer DEFAULT 0 NOT NULL,
    nextattempt timestamp without time zone NOT NULL,
    numfailures integer DEFAULT 0 NOT NULL,
    flags integer DEFAULT 0 NOT NULL,
    CONSTRAINT peers_numfailures_check CHECK ((numfailures >= 0)),
    CONSTRAINT peers_port_check CHECK (((port > 0) AND (port <= 65535)))
);


--
-- Name: publishqueue; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.publishqueue (
    ledger integer NOT NULL,
    state text
);


--
-- Name: pubsub; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pubsub (
    resid character(32) NOT NULL,
    lastread integer
);


--
-- Name: scphistory; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scphistory (
    nodeid character(56) NOT NULL,
    ledgerseq integer NOT NULL,
    envelope text NOT NULL,
    CONSTRAINT scphistory_ledgerseq_check CHECK ((ledgerseq >= 0))
);


--
-- Name: scpquorums; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.scpquorums (
    qsethash character(64) NOT NULL,
    lastledgerseq integer NOT NULL,
    qset text NOT NULL,
    CONSTRAINT scpquorums_lastledgerseq_check CHECK ((lastledgerseq >= 0))
);


--
-- Name: signers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.signers (
    accountid character varying(56) NOT NULL,
    publickey character varying(56) NOT NULL,
    weight integer NOT NULL
);


--
-- Name: storestate; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.storestate (
    statename character(32) NOT NULL,
    state text
);


--
-- Name: trustlines; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trustlines (
    accountid character varying(56) NOT NULL,
    assettype integer NOT NULL,
    issuer character varying(56) NOT NULL,
    assetcode character varying(12) NOT NULL,
    tlimit bigint NOT NULL,
    balance bigint NOT NULL,
    flags integer NOT NULL,
    lastmodified integer NOT NULL,
    CONSTRAINT trustlines_balance_check CHECK ((balance >= 0)),
    CONSTRAINT trustlines_tlimit_check CHECK ((tlimit > 0))
);


--
-- Name: txfeehistory; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.txfeehistory (
    txid character(64) NOT NULL,
    ledgerseq integer NOT NULL,
    txindex integer NOT NULL,
    txchanges text NOT NULL,
    CONSTRAINT txfeehistory_ledgerseq_check CHECK ((ledgerseq >= 0))
);


--
-- Name: txhistory; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.txhistory (
    txid character(64) NOT NULL,
    ledgerseq integer NOT NULL,
    txindex integer NOT NULL,
    txbody text NOT NULL,
    txresult text NOT NULL,
    txmeta text NOT NULL,
    CONSTRAINT txhistory_ledgerseq_check CHECK ((ledgerseq >= 0))
);


--
-- Data for Name: accountdata; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.accountdata VALUES ('GD6NTRJW5Z6NCWH4USWMNEYF77RUR2MTO6NP4KEDVJATTCUXDRO3YIFS', 'done', 'dHJ1ZQ==', 5);


--
-- Data for Name: accounts; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.accounts VALUES ('GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H', 999999989999999900, 1, 0, NULL, '', 'AQAAAA==', 0, 3);
INSERT INTO public.accounts VALUES ('GD6NTRJW5Z6NCWH4USWMNEYF77RUR2MTO6NP4KEDVJATTCUXDRO3YIFS', 9999999800, 12884901890, 2, NULL, '', 'AQAAAA==', 0, 5);


--
-- Data for Name: ban; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: ledgerheaders; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.ledgerheaders VALUES ('63d98f536ee68d1b27b5b89f23af5311b7569a24faf1403ad0b52b633b07be99', '0000000000000000000000000000000000000000000000000000000000000000', '572a2e32ff248a07b0e70fd1f6d318c1facd20b6cc08c33d5775259868125a16', 1, 0, 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABXKi4y/ySKB7DnD9H20xjB+s0gtswIwz1XdSWYaBJaFgAAAAEN4Lazp2QAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAZAX14QAAAABkAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA');
INSERT INTO public.ledgerheaders VALUES ('60df958275532e4fe67d71a1cb3cd8de663a8c3b5a223474dab0e7b3c475bff0', '63d98f536ee68d1b27b5b89f23af5311b7569a24faf1403ad0b52b633b07be99', '735227ed398461291237687b08446aa2c9b096e0c98a462dadda569f05dd2484', 2, 1533068144, 'AAAACmPZj1Nu5o0bJ7W4nyOvUxG3Vpok+vFAOtC1K2M7B76ZuZRHr9UdXKbTKiclfOjy72YZFJUkJPVcKT5htvorm1QAAAAAW2DDcAAAAAIAAAAIAAAAAQAAAAoAAAAIAAAAAwAAJxAAAAAA3z9hmASpL9tAVxktxD3XSOp3itxSvEmM6AUkwBS4ERlzUiftOYRhKRI3aHsIRGqiybCW4MmKRi2t2lafBd0khAAAAAIN4Lazp2QAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAZAX14QAAACcQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA');
INSERT INTO public.ledgerheaders VALUES ('f8a6f29f9fecaf78888c2ce9f5393be527d80902ea23e280471c7a0bd0d9044c', '60df958275532e4fe67d71a1cb3cd8de663a8c3b5a223474dab0e7b3c475bff0', 'da14da6b0c81b426f580632f565e93a528b11589dc1aee5f5c4e23a0f1eaaa7d', 3, 1533068145, 'AAAACmDflYJ1Uy5P5n1xocs82N5mOow7WiI0dNqw57PEdb/wvEIDnADpgTcywCkoZot/vEX38ZM5ldsUGRNbCauqKUgAAAAAW2DDcQAAAAAAAAAAlzJ1vISHXzElAf05LhN7qiqWqKvjHhTijb/BgG6FsuLaFNprDIG0JvWAYy9WXpOlKLEVidwa7l9cTiOg8eqqfQAAAAMN4Lazp2QAAAAAAAAAAABkAAAAAAAAAAAAAAAAAAAAZAX14QAAACcQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA');
INSERT INTO public.ledgerheaders VALUES ('53d86500c64978b1812d1cd9ca2a162e0256cae7e3e38c980dcd7492e3e68d17', 'f8a6f29f9fecaf78888c2ce9f5393be527d80902ea23e280471c7a0bd0d9044c', 'bf7edb6c1979a8b7b7c4255be766c86dcaac665c236df658df13d39a27eafcba', 4, 1533068146, 'AAAACvim8p+f7K94iIws6fU5O+Un2AkC6iPigEccegvQ2QRMudrOmtAleQgKObpknqIXaKQ5qip8vqkTFAfcNMcIptwAAAAAW2DDcgAAAAAAAAAApOCJokUPy1wA/XbpkumCKr9Nv3B8+fRGt4d1ygPrKBy/fttsGXmot7fEJVvnZshtyqxmXCNt9ljfE9OaJ+r8ugAAAAQN4Lazp2QAAAAAAAAAAADIAAAAAAAAAAAAAAAAAAAAZAX14QAAACcQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA');
INSERT INTO public.ledgerheaders VALUES ('f8b11203795590c51283eac2dc0033231c87617ffb4631c2139412f009b09b52', '53d86500c64978b1812d1cd9ca2a162e0256cae7e3e38c980dcd7492e3e68d17', 'a486f9d25d7c2d7148e558cea733112d5c6e2f0c31476e980f9461fb553d4d43', 5, 1533068147, 'AAAAClPYZQDGSXixgS0c2coqFi4CVsrn4+OMmA3NdJLj5o0Xr6qVvhgiPPAJ8OtHdZAkWLL6WwzsLBPnTxt46pIc8zwAAAAAW2DDcwAAAAAAAAAAnzt6UJX5by6BwcesLvsJTMJPn/Y/s/aBHlEOtmT5mwekhvnSXXwtcUjlWM6nMxEtXG4vDDFHbpgPlGH7VT1NQwAAAAUN4Lazp2QAAAAAAAAAAAEsAAAAAAAAAAAAAAAAAAAAZAX14QAAACcQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA');
INSERT INTO public.ledgerheaders VALUES ('26ad191484004d151886bbcc1ff6b283374dc290abd805007174d618126aa092', 'f8b11203795590c51283eac2dc0033231c87617ffb4631c2139412f009b09b52', 'c2d5be06f487f5351e3376163c5166f367883682e402f702329a8f1e8a4397b2', 6, 1533068148, 'AAAACvixEgN5VZDFEoPqwtwAMyMch2F/+0YxwhOUEvAJsJtSjuogCl5wiAZ+AOA/03xXeEF1sUkCkIbtn7O0E7duDLUAAAAAW2DDdAAAAAAAAAAA3z9hmASpL9tAVxktxD3XSOp3itxSvEmM6AUkwBS4ERnC1b4G9If1NR4zdhY8UWbzZ4g2guQC9wIymo8eikOXsgAAAAYN4Lazp2QAAAAAAAAAAAEsAAAAAAAAAAAAAAAAAAAAZAX14QAAACcQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA');


--
-- Data for Name: offers; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: peers; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: publishqueue; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: pubsub; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scphistory; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.scphistory VALUES ('GBUHK6B74FLHPGXKG435FVCZUPN4ORBCEMZKDOQXFSUVFVYCNCUE32A4', 2, 'AAAAAGh1eD/hVnea6jc30tRZo9vHRCIjMqG6FyypUtcCaKhNAAAAAAAAAAIAAAACAAAAAQAAAEi5lEev1R1cptMqJyV86PLvZhkUlSQk9VwpPmG2+iubVAAAAABbYMNwAAAAAgAAAAgAAAABAAAACgAAAAgAAAADAAAnEAAAAAAAAAABQyFLDlLr0IjToYwqaI1fov0vHiDmMl6qmtp4K/sqmYMAAABAPr1gZq0kCx3m+4CEiXdRmAs32kPxfRQU0zu8VXmgU7h7xCjZfKON9IbWUBlznRmWHhLevlgYjAWjyh2Ca9WBAA==');
INSERT INTO public.scphistory VALUES ('GBUHK6B74FLHPGXKG435FVCZUPN4ORBCEMZKDOQXFSUVFVYCNCUE32A4', 3, 'AAAAAGh1eD/hVnea6jc30tRZo9vHRCIjMqG6FyypUtcCaKhNAAAAAAAAAAMAAAACAAAAAQAAADC8QgOcAOmBNzLAKShmi3+8RffxkzmV2xQZE1sJq6opSAAAAABbYMNxAAAAAAAAAAAAAAABQyFLDlLr0IjToYwqaI1fov0vHiDmMl6qmtp4K/sqmYMAAABAahLF+bu41V5FnBw8/4FAcy/4lPhywXx/5c3p1Es9m+a33l98wJHULoEGNdNHmtyltwBScxHUHmydobFIcPuYBw==');
INSERT INTO public.scphistory VALUES ('GBUHK6B74FLHPGXKG435FVCZUPN4ORBCEMZKDOQXFSUVFVYCNCUE32A4', 4, 'AAAAAGh1eD/hVnea6jc30tRZo9vHRCIjMqG6FyypUtcCaKhNAAAAAAAAAAQAAAACAAAAAQAAADC52s6a0CV5CAo5umSeohdopDmqKny+qRMUB9w0xwim3AAAAABbYMNyAAAAAAAAAAAAAAABQyFLDlLr0IjToYwqaI1fov0vHiDmMl6qmtp4K/sqmYMAAABAzPyMPtHrW5yYO4geQBV09NRPzVpJBdz8H8MzoHMeC6PZm/Oor0+VGicIeYsoyA+7qvFsu/8QV0kWJnECpGgHCQ==');
INSERT INTO public.scphistory VALUES ('GBUHK6B74FLHPGXKG435FVCZUPN4ORBCEMZKDOQXFSUVFVYCNCUE32A4', 5, 'AAAAAGh1eD/hVnea6jc30tRZo9vHRCIjMqG6FyypUtcCaKhNAAAAAAAAAAUAAAACAAAAAQAAADCvqpW+GCI88Anw60d1kCRYsvpbDOwsE+dPG3jqkhzzPAAAAABbYMNzAAAAAAAAAAAAAAABQyFLDlLr0IjToYwqaI1fov0vHiDmMl6qmtp4K/sqmYMAAABAe9/JA8N0X5k3Vy4i647xvUxY13U38GcbKOCR4Sxt3qOlWX+nddWeGSqH7eWhWbFP5XqFuWevCQepXOCQBsYmDw==');
INSERT INTO public.scphistory VALUES ('GBUHK6B74FLHPGXKG435FVCZUPN4ORBCEMZKDOQXFSUVFVYCNCUE32A4', 6, 'AAAAAGh1eD/hVnea6jc30tRZo9vHRCIjMqG6FyypUtcCaKhNAAAAAAAAAAYAAAACAAAAAQAAADCO6iAKXnCIBn4A4D/TfFd4QXWxSQKQhu2fs7QTt24MtQAAAABbYMN0AAAAAAAAAAAAAAABQyFLDlLr0IjToYwqaI1fov0vHiDmMl6qmtp4K/sqmYMAAABAm1gEbLT8PoQlbmNFsU70F+P219G/89LHUi5va/4ZAEY/RxhSFDSfCII60WC+DHSvbRcT5plooZvwWHEKMrJnDw==');


--
-- Data for Name: scpquorums; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.scpquorums VALUES ('43214b0e52ebd088d3a18c2a688d5fa2fd2f1e20e6325eaa9ada782bfb2a9983', 6, 'AAAAAQAAAAEAAAAAaHV4P+FWd5rqNzfS1Fmj28dEIiMyoboXLKlS1wJoqE0AAAAA');


--
-- Data for Name: signers; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.signers VALUES ('GD6NTRJW5Z6NCWH4USWMNEYF77RUR2MTO6NP4KEDVJATTCUXDRO3YIFS', 'XC6GFVFYBWPDNWRJYFWF2TM7CFZR6NQFFRZEAGTWYI6A7NNJW5CCGCON', 1);


--
-- Data for Name: storestate; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.storestate VALUES ('databaseschema                  ', '6');
INSERT INTO public.storestate VALUES ('networkpassphrase               ', 'Test SDF Network ; September 2015');
INSERT INTO public.storestate VALUES ('forcescponnextlaunch            ', 'false');
INSERT INTO public.storestate VALUES ('ledgerupgrades                  ', '{
    "time": 0,
    "version": {
        "has": false
    },
    "fee": {
        "has": false
    },
    "maxtxsize": {
        "has": false
    },
    "reserve": {
        "has": false
    }
}');
INSERT INTO public.storestate VALUES ('lastclosedledger                ', '26ad191484004d151886bbcc1ff6b283374dc290abd805007174d618126aa092');
INSERT INTO public.storestate VALUES ('historyarchivestate             ', '{
    "version": 1,
    "server": "v9.2.0-260-gce5e5d10",
    "currentLedger": 6,
    "currentBuckets": [
        {
            "curr": "0000000000000000000000000000000000000000000000000000000000000000",
            "next": {
                "state": 0
            },
            "snap": "aaa4881b8995adbb5ab737472f1af7f351c296fe6dd094ff84e6c630bd9037ff"
        },
        {
            "curr": "8aa5e6da375080b09395c35b99f54b16e836d297fe492cab9b94fa3fa1cf7d52",
            "next": {
                "state": 1,
                "output": "aaa4881b8995adbb5ab737472f1af7f351c296fe6dd094ff84e6c630bd9037ff"
            },
            "snap": "0000000000000000000000000000000000000000000000000000000000000000"
        },
        {
            "curr": "0000000000000000000000000000000000000000000000000000000000000000",
            "next": {
                "state": 0
            },
            "snap": "0000000000000000000000000000000000000000000000000000000000000000"
        },
        {
            "curr": "0000000000000000000000000000000000000000000000000000000000000000",
            "next": {
                "state": 0
            },
            "snap": "0000000000000000000000000000000000000000000000000000000000000000"
        },
        {
            "curr": "0000000000000000000000000000000000000000000000000000000000000000",
            "next": {
                "state": 0
            },
            "snap": "0000000000000000000000000000000000000000000000000000000000000000"
        },
        {
            "curr": "0000000000000000000000000000000000000000000000000000000000000000",
            "next": {
                "state": 0
            },
            "snap": "0000000000000000000000000000000000000000000000000000000000000000"
        },
        {
            "curr": "0000000000000000000000000000000000000000000000000000000000000000",
            "next": {
                "state": 0
            },
            "snap": "0000000000000000000000000000000000000000000000000000000000000000"
        },
        {
            "curr": "0000000000000000000000000000000000000000000000000000000000000000",
            "next": {
                "state": 0
            },
            "snap": "0000000000000000000000000000000000000000000000000000000000000000"
        },
        {
            "curr": "0000000000000000000000000000000000000000000000000000000000000000",
            "next": {
                "state": 0
            },
            "snap": "0000000000000000000000000000000000000000000000000000000000000000"
        },
        {
            "curr": "0000000000000000000000000000000000000000000000000000000000000000",
            "next": {
                "state": 0
            },
            "snap": "0000000000000000000000000000000000000000000000000000000000000000"
        },
        {
            "curr": "0000000000000000000000000000000000000000000000000000000000000000",
            "next": {
                "state": 0
            },
            "snap": "0000000000000000000000000000000000000000000000000000000000000000"
        }
    ]
}');
INSERT INTO public.storestate VALUES ('lastscpdata                     ', 'AAAAAgAAAABodXg/4VZ3muo3N9LUWaPbx0QiIzKhuhcsqVLXAmioTQAAAAAAAAAGAAAAA0MhSw5S69CI06GMKmiNX6L9Lx4g5jJeqpraeCv7KpmDAAAAAQAAADCO6iAKXnCIBn4A4D/TfFd4QXWxSQKQhu2fs7QTt24MtQAAAABbYMN0AAAAAAAAAAAAAAABAAAAMI7qIApecIgGfgDgP9N8V3hBdbFJApCG7Z+ztBO3bgy1AAAAAFtgw3QAAAAAAAAAAAAAAECNmdr6PQwPqqzkk5txyfcMXGhJcSR84wrCVxMkrB2aqUscvMpUfmc2tViy+rZcTBwUNu1idQEDEPLK3yLEFZMFAAAAAGh1eD/hVnea6jc30tRZo9vHRCIjMqG6FyypUtcCaKhNAAAAAAAAAAYAAAACAAAAAQAAADCO6iAKXnCIBn4A4D/TfFd4QXWxSQKQhu2fs7QTt24MtQAAAABbYMN0AAAAAAAAAAAAAAABQyFLDlLr0IjToYwqaI1fov0vHiDmMl6qmtp4K/sqmYMAAABAm1gEbLT8PoQlbmNFsU70F+P219G/89LHUi5va/4ZAEY/RxhSFDSfCII60WC+DHSvbRcT5plooZvwWHEKMrJnDwAAAAH4sRIDeVWQxRKD6sLcADMjHIdhf/tGMcITlBLwCbCbUgAAAAAAAAABAAAAAQAAAAEAAAAAaHV4P+FWd5rqNzfS1Fmj28dEIiMyoboXLKlS1wJoqE0AAAAA');


--
-- Data for Name: trustlines; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: txfeehistory; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.txfeehistory VALUES ('54c49533d937a906c0e6e501322bb600ffe332bf888cb474bd4261d42f542470', 3, 1, 'AAAAAgAAAAMAAAABAAAAAAAAAABi/B0L0JGythwN1lY0aypo19NHxvLCyO5tBEcCVvwF9w3gtrOnZAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAAAADAAAAAAAAAABi/B0L0JGythwN1lY0aypo19NHxvLCyO5tBEcCVvwF9w3gtrOnY/+cAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==');
INSERT INTO public.txfeehistory VALUES ('59de4450bcc1830b6da83fb481aab833db04dadf1e88d568a00274d4038d531e', 4, 1, 'AAAAAgAAAAMAAAADAAAAAAAAAAD82cU27nzRWPykrMaTBf/jSOmTd5r+KIOqQTmKlxxdvAAAAAJUC+QAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAAAAEAAAAAAAAAAD82cU27nzRWPykrMaTBf/jSOmTd5r+KIOqQTmKlxxdvAAAAAJUC+OcAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==');
INSERT INTO public.txfeehistory VALUES ('b0879add9f3957c9796d1d1fb23720dbed15d07793c742773455f2706c0e9a25', 5, 1, 'AAAAAgAAAAMAAAAEAAAAAAAAAAD82cU27nzRWPykrMaTBf/jSOmTd5r+KIOqQTmKlxxdvAAAAAJUC+OcAAAAAwAAAAEAAAABAAAAAAAAAAAAAAAAAQAAAAAAAAEAAAACvGLUuA2eNtopwWxdTZ8Rcx82BSxyQBp2wjwPtam3RCMAAAABAAAAAAAAAAAAAAABAAAABQAAAAAAAAAA/NnFNu580Vj8pKzGkwX/40jpk3ea/iiDqkE5ipccXbwAAAACVAvjOAAAAAMAAAABAAAAAQAAAAAAAAAAAAAAAAEAAAAAAAABAAAAArxi1LgNnjbaKcFsXU2fEXMfNgUsckAadsI8D7Wpt0QjAAAAAQAAAAAAAAAA');


--
-- Data for Name: txhistory; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.txhistory VALUES ('54c49533d937a906c0e6e501322bb600ffe332bf888cb474bd4261d42f542470', 3, 1, 'AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAAZAAAAAAAAAABAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAA/NnFNu580Vj8pKzGkwX/40jpk3ea/iiDqkE5ipccXbwAAAACVAvkAAAAAAAAAAABVvwF9wAAAEDi0+98/ltS2PZOxNCNogdC0ctkOWrNnQ+3eVu2PI3+LNdVssYOrw4gwvZFULsMpS166y7rVfyn6AIp7gqV5pMD', 'VMSVM9k3qQbA5uUBMiu2AP/jMr+IjLR0vUJh1C9UJHAAAAAAAAAAZAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAA==', 'AAAAAQAAAAIAAAADAAAAAwAAAAAAAAAAYvwdC9CRsrYcDdZWNGsqaNfTR8bywsjubQRHAlb8BfcN4Lazp2P/nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAwAAAAAAAAAAYvwdC9CRsrYcDdZWNGsqaNfTR8bywsjubQRHAlb8BfcN4Lazp2P/nAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAwAAAAAAAAADAAAAAAAAAAD82cU27nzRWPykrMaTBf/jSOmTd5r+KIOqQTmKlxxdvAAAAAJUC+QAAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMAAAADAAAAAAAAAABi/B0L0JGythwN1lY0aypo19NHxvLCyO5tBEcCVvwF9w3gtrOnY/+cAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAAAADAAAAAAAAAABi/B0L0JGythwN1lY0aypo19NHxvLCyO5tBEcCVvwF9w3gtrFTWBucAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==');
INSERT INTO public.txhistory VALUES ('59de4450bcc1830b6da83fb481aab833db04dadf1e88d568a00274d4038d531e', 4, 1, 'AAAAAPzZxTbufNFY/KSsxpMF/+NI6ZN3mv4og6pBOYqXHF28AAAAZAAAAAMAAAABAAAAAAAAAAAAAAABAAAAAAAAAAUAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAK8YtS4DZ422inBbF1NnxFzHzYFLHJAGnbCPA+1qbdEIwAAAAEAAAAAAAAAAZccXbwAAABAeVu+uPT8KOhoKoNJidCWqVs71WAIqQns2Zq4mM3LBluMVDHej/SJhUxiKlsSR5MJwQU3trQbPsOAwb56BinbCw==', 'Wd5EULzBgwttqD+0gaq4M9sE2t8eiNVooAJ01AONUx4AAAAAAAAAZAAAAAAAAAABAAAAAAAAAAUAAAAAAAAAAA==', 'AAAAAQAAAAIAAAADAAAABAAAAAAAAAAA/NnFNu580Vj8pKzGkwX/40jpk3ea/iiDqkE5ipccXbwAAAACVAvjnAAAAAMAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABAAAAAAAAAAA/NnFNu580Vj8pKzGkwX/40jpk3ea/iiDqkE5ipccXbwAAAACVAvjnAAAAAMAAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAgAAAAMAAAAEAAAAAAAAAAD82cU27nzRWPykrMaTBf/jSOmTd5r+KIOqQTmKlxxdvAAAAAJUC+OcAAAAAwAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEAAAAEAAAAAAAAAAD82cU27nzRWPykrMaTBf/jSOmTd5r+KIOqQTmKlxxdvAAAAAJUC+OcAAAAAwAAAAEAAAABAAAAAAAAAAAAAAAAAQAAAAAAAAEAAAACvGLUuA2eNtopwWxdTZ8Rcx82BSxyQBp2wjwPtam3RCMAAAABAAAAAAAAAAA=');
INSERT INTO public.txhistory VALUES ('b0879add9f3957c9796d1d1fb23720dbed15d07793c742773455f2706c0e9a25', 5, 1, 'AAAAAPzZxTbufNFY/KSsxpMF/+NI6ZN3mv4og6pBOYqXHF28AAAAZAAAAAMAAAACAAAAAAAAAAAAAAABAAAAAAAAAAoAAAAEZG9uZQAAAAEAAAAEdHJ1ZQAAAAAAAAABqbdEIwAAACC5TSe5k00+CKUuUtfafav6xITv43pTgO6QiPes4u/N6Q==', 'sIea3Z85V8l5bR0fsjcg2+0V0HeTx0J3NFXycGwOmiUAAAAAAAAAZAAAAAAAAAABAAAAAAAAAAoAAAAAAAAAAA==', 'AAAAAQAAAAIAAAADAAAABQAAAAAAAAAA/NnFNu580Vj8pKzGkwX/40jpk3ea/iiDqkE5ipccXbwAAAACVAvjOAAAAAMAAAABAAAAAQAAAAAAAAAAAAAAAAEAAAAAAAABAAAAArxi1LgNnjbaKcFsXU2fEXMfNgUsckAadsI8D7Wpt0QjAAAAAQAAAAAAAAAAAAAAAQAAAAUAAAAAAAAAAPzZxTbufNFY/KSsxpMF/+NI6ZN3mv4og6pBOYqXHF28AAAAAlQL4zgAAAADAAAAAgAAAAEAAAAAAAAAAAAAAAABAAAAAAAAAQAAAAK8YtS4DZ422inBbF1NnxFzHzYFLHJAGnbCPA+1qbdEIwAAAAEAAAAAAAAAAAAAAAEAAAADAAAAAAAAAAUAAAADAAAAAPzZxTbufNFY/KSsxpMF/+NI6ZN3mv4og6pBOYqXHF28AAAABGRvbmUAAAAEdHJ1ZQAAAAAAAAAAAAAAAwAAAAUAAAAAAAAAAPzZxTbufNFY/KSsxpMF/+NI6ZN3mv4og6pBOYqXHF28AAAAAlQL4zgAAAADAAAAAgAAAAEAAAAAAAAAAAAAAAABAAAAAAAAAQAAAAK8YtS4DZ422inBbF1NnxFzHzYFLHJAGnbCPA+1qbdEIwAAAAEAAAAAAAAAAAAAAAEAAAAFAAAAAAAAAAD82cU27nzRWPykrMaTBf/jSOmTd5r+KIOqQTmKlxxdvAAAAAJUC+M4AAAAAwAAAAIAAAACAAAAAAAAAAAAAAAAAQAAAAAAAAEAAAACvGLUuA2eNtopwWxdTZ8Rcx82BSxyQBp2wjwPtam3RCMAAAABAAAAAAAAAAA=');


--
-- Name: accountdata accountdata_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.accountdata
    ADD CONSTRAINT accountdata_pkey PRIMARY KEY (accountid, dataname);


--
-- Name: accounts accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.accounts
    ADD CONSTRAINT accounts_pkey PRIMARY KEY (accountid);


--
-- Name: ban ban_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ban
    ADD CONSTRAINT ban_pkey PRIMARY KEY (nodeid);


--
-- Name: ledgerheaders ledgerheaders_ledgerseq_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledgerheaders
    ADD CONSTRAINT ledgerheaders_ledgerseq_key UNIQUE (ledgerseq);


--
-- Name: ledgerheaders ledgerheaders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ledgerheaders
    ADD CONSTRAINT ledgerheaders_pkey PRIMARY KEY (ledgerhash);


--
-- Name: offers offers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.offers
    ADD CONSTRAINT offers_pkey PRIMARY KEY (offerid);


--
-- Name: peers peers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.peers
    ADD CONSTRAINT peers_pkey PRIMARY KEY (ip, port);


--
-- Name: publishqueue publishqueue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publishqueue
    ADD CONSTRAINT publishqueue_pkey PRIMARY KEY (ledger);


--
-- Name: pubsub pubsub_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pubsub
    ADD CONSTRAINT pubsub_pkey PRIMARY KEY (resid);


--
-- Name: scpquorums scpquorums_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.scpquorums
    ADD CONSTRAINT scpquorums_pkey PRIMARY KEY (qsethash);


--
-- Name: signers signers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.signers
    ADD CONSTRAINT signers_pkey PRIMARY KEY (accountid, publickey);


--
-- Name: storestate storestate_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.storestate
    ADD CONSTRAINT storestate_pkey PRIMARY KEY (statename);


--
-- Name: trustlines trustlines_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trustlines
    ADD CONSTRAINT trustlines_pkey PRIMARY KEY (accountid, issuer, assetcode);


--
-- Name: txfeehistory txfeehistory_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.txfeehistory
    ADD CONSTRAINT txfeehistory_pkey PRIMARY KEY (ledgerseq, txindex);


--
-- Name: txhistory txhistory_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.txhistory
    ADD CONSTRAINT txhistory_pkey PRIMARY KEY (ledgerseq, txindex);


--
-- Name: accountbalances; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX accountbalances ON public.accounts USING btree (balance) WHERE (balance >= 1000000000);


--
-- Name: buyingissuerindex; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX buyingissuerindex ON public.offers USING btree (buyingissuer);


--
-- Name: histbyseq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX histbyseq ON public.txhistory USING btree (ledgerseq);


--
-- Name: histfeebyseq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX histfeebyseq ON public.txfeehistory USING btree (ledgerseq);


--
-- Name: ledgersbyseq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ledgersbyseq ON public.ledgerheaders USING btree (ledgerseq);


--
-- Name: priceindex; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX priceindex ON public.offers USING btree (price);


--
-- Name: scpenvsbyseq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX scpenvsbyseq ON public.scphistory USING btree (ledgerseq);


--
-- Name: scpquorumsbyseq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX scpquorumsbyseq ON public.scpquorums USING btree (lastledgerseq);


--
-- Name: sellingissuerindex; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sellingissuerindex ON public.offers USING btree (sellingissuer);


--
-- Name: signersaccount; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX signersaccount ON public.signers USING btree (accountid);


--
-- PostgreSQL database dump complete
--


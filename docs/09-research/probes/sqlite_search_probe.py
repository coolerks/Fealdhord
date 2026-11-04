"""离线验证 FTS tokenizer 边界；不是未来 Go 驱动或产品性能认证。"""
import json
import sqlite3


def cjk_terms(text):
    """探针只处理测试中文范围；产品需另测完整 Unicode 与混合分段。"""
    runs, current = [], ""
    for ch in text:
        if "\u4e00" <= ch <= "\u9fff":
            current += ch
        else:
            if current:
                runs.append(current)
                current = ""
    if current:
        runs.append(current)
    terms = []
    for run in runs:
        terms.extend("单" + ch for ch in run)
        terms.extend("双" + run[i:i + 2] for i in range(len(run) - 1))
    return " ".join(terms)


def main():
    db = sqlite3.connect(":memory:")
    db.execute("CREATE VIRTUAL TABLE words USING fts5(body, tokenize='unicode61')")
    db.execute("CREATE VIRTUAL TABLE triples USING fts5(body, tokenize='trigram')")
    db.execute("CREATE VIRTUAL TABLE pairs USING fts5(body, terms, tokenize='unicode61')")
    body = "折栈支持中文并发阅读"
    db.execute("INSERT INTO words VALUES (?)", (body,))
    db.execute("INSERT INTO triples VALUES (?)", (body,))
    db.execute("INSERT INTO pairs VALUES (?, ?)", (body, cjk_terms(body)))
    def count(table, query):
        # 表名仅来自下方固定探针，不接受外部输入。
        return db.execute(f"SELECT count(*) FROM {table} WHERE {table} MATCH ?", (query,)).fetchone()[0]
    cases = {
        "unicode61_双字子串": count("words", '"并发"'),
        "trigram_双字子串": count("triples", '"并发"'),
        "trigram_三字子串": count("triples", '"中文并"'),
        "候选二字token": count("pairs", 'terms:"双并发"'),
        "候选单字token": count("pairs", 'terms:"单栈"'),
    }
    assert list(cases.values()) == [0, 0, 1, 1, 1], cases
    db.execute("CREATE TABLE memberships(article_id TEXT, category_id TEXT, source TEXT CHECK(source IN ('manual','feed','ai')), state TEXT CHECK(state IN ('included','excluded')), PRIMARY KEY(article_id,category_id,source))")
    db.executemany("INSERT INTO memberships VALUES ('a','c',?,?)", [("feed", "included"), ("ai", "included"), ("manual", "excluded")])
    rows = dict(db.execute("SELECT source,state FROM memberships WHERE article_id='a' AND category_id='c'"))
    effective = rows.get("manual") == "included" if "manual" in rows else any(v == "included" for v in rows.values())
    assert len(rows) == 3 and effective is False
    print(json.dumps({"日期": "2026-11-04", "运行时": "Python sqlite3 内存库", "sqlite_version": sqlite3.sqlite_version, "断言": cases, "三来源独立保存": len(rows) == 3, "manual_excluded_有效成员": effective, "限制": "仅语义探针；未测试 Go 驱动、索引体积、全文连续性过滤或 100k 性能"}, ensure_ascii=False, indent=2))
    db.close()


if __name__ == "__main__":
    main()

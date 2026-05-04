from __future__ import annotations

from pathlib import Path


EXCLUDED_PARTS = {
    "__pycache__",
    "coverage",
    "dist",
    "build",
    "graphify-out",
    "node_modules",
}


def _is_repo_code(path: Path) -> bool:
    parts = set(path.parts)
    if parts & EXCLUDED_PARTS:
        return False
    if any(part.startswith(".") for part in path.parts):
        return False
    return True


def main() -> int:
    from graphify.analyze import god_nodes, suggest_questions, surprising_connections
    from graphify.build import build_from_json
    from graphify.cluster import cluster, score_all
    from graphify.export import to_json
    from graphify.extract import collect_files, extract
    from graphify.report import generate

    root = Path(".").resolve()
    collected = collect_files(root)
    code_files = [path for path in collected if _is_repo_code(path.relative_to(root))]
    skipped = len(collected) - len(code_files)

    if not code_files:
        print("[graphify refresh] No repo code files found.")
        return 1

    print(f"[graphify refresh] Code files: {len(code_files)} kept, {skipped} generated/vendor files skipped")
    result = extract(code_files)

    detection = {
        "files": {"code": [str(path.relative_to(root)) for path in code_files], "document": [], "paper": [], "image": []},
        "total_files": len(code_files),
        "total_words": 0,
    }

    graph = build_from_json(result)
    communities = cluster(graph)
    cohesion = score_all(graph, communities)
    gods = god_nodes(graph)
    surprises = surprising_connections(graph, communities)
    labels = {cid: f"Community {cid}" for cid in communities}
    questions = suggest_questions(graph, communities, labels)

    out = root / "graphify-out"
    out.mkdir(exist_ok=True)
    report = generate(
        graph,
        communities,
        cohesion,
        labels,
        gods,
        surprises,
        detection,
        {"input": 0, "output": 0},
        str(root),
        suggested_questions=questions,
    )
    (out / "GRAPH_REPORT.md").write_text(report, encoding="utf-8")
    to_json(graph, communities, str(out / "graph.json"))

    flag = out / "needs_update"
    if flag.exists():
        flag.unlink()

    print(
        f"[graphify refresh] Rebuilt: {graph.number_of_nodes()} nodes, "
        f"{graph.number_of_edges()} edges, {len(communities)} communities"
    )
    print(f"[graphify refresh] graph.json and GRAPH_REPORT.md updated in {out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

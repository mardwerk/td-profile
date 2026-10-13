# Profile validation research: game design research 2026 09 22

Source investigation recorded September 22, 2026. "Inspected" means that relevant primary text was accessed; experiments were not reproduced. The source text was edited later, so the inspection date is not a last-edit date. The separate September 26 review checked metadata and selected software examples; it did not reread the paper findings. No new literature investigation is claimed here.

These are dated source observations and proposals. Preserve their recorded review and rejection status. Current local contracts govern acceptance; these notes neither select implementation nor resume paused development.

### R15. Ludax

Graham Todd et al. **Ludax: A GPU-Accelerated Domain Specific Language for Board Games.** [arXiv:2506.22609](https://arxiv.org/abs/2506.22609), first submitted in 2025; [v2 inspected](https://arxiv.org/html/2506.22609v2), March 25, 2026. Inspected: representation, compilation, supported game class and references. R12 cites the earlier 2025 version; this record deliberately distinguishes it from the newer text inspected.

Ludax compiles formal board-game descriptions into JAX environments for accelerated simulation. The inspected version supports two-player, perfect-information, turn-based board games built from placing, moving and capturing pieces.

Adaptation: take the representation lesson: executable meaning comes from defined operators and their implementation. Extending the vocabulary changes the expressible design space. A new term in a specification is not a new implemented operator.

Limits: this is not a ready-made real-time tower defense runtime. Its performance claims should not be transferred to our event model, maps or agents. Consider runtime semantics before acceleration or a new language dependency.

### R21. Formal mechanic generation

Alexander Zook and Mark O. Riedl. **Automatic Game Design via Mechanic Generation.** AAAI 2014. [arXiv:1908.01420v1](https://arxiv.org/html/1908.01420v1), [DOI:10.1609/aaai.v28i1.8788](https://doi.org/10.1609/aaai.v28i1.8788). Inspected: representation, constraint-based generation, planning and demonstration domains. The arXiv deposit is 2019; the paper is from 2014. Promoted from the original R3/R7 citation lead.

A constraint solver generates mechanics expressed through planning-related actions. A planner checks playability requirements. Examples include role-playing, platforming and a combined setting. The work distinguishes constraints on what a mechanic is from constraints on what play it permits.

Adaptation: express preconditions, resource changes and effects so we can ask whether a proposed tactical outcome is reachable. Check whether a new ability bypasses an intended prerequisite. Formal reachability can support a mechanic claim within the encoded model; it cannot establish strategic depth, readability or fun.

## Source identifier references

These publication identities supply local citation context. Inspection and reproduction limits are recorded above.

- R1. Kynan Sorochan and Matthew Guzdial. **Generating Real-Time Strategy Game Units Using Search-Based Procedural Content Generation and Monte Carlo Tree Search.** Experimental AI in Games Workshop, 2022. [arXiv:2212.03387v1](https://arxiv.org/html/2212.03387v1). Inspected: Search Space, Evaluator, Search Method, generated units, evaluation and references.
- R3. Graham Todd, Alexander G. Padula, Matthew Stephenson, Éric Piette, Dennis J. N. J. Soemers and Julian Togelius. **GAVEL: Generating Games Via Evolution and Language Models.** NeurIPS 2024. [arXiv:2407.09388v2](https://arxiv.org/html/2407.09388v2). Inspected: representation, mutation, evaluation, experiments, results, limitations and references. [Code link](https://github.com/gdrtodd/gavel), not executed in this investigation.
- R7. Alexander Zook, Brent Harrison and Mark O. Riedl. **Monte-Carlo Tree Search for Simulation-based Strategy Analysis.** The paper identifies FDG 2015 as its conference; the arXiv deposit is 2019. [arXiv:1908.01423v1](https://arxiv.org/html/1908.01423v1). Inspected: methodology, skill-based metrics, case studies, limitations and references. R1 cites the deposit year.
- R12. Katherine M. Collins et al. **Generation and Evaluation in the Human Invention Process through the Lens of Game Design.** CogSci 2025. [arXiv:2508.10914v1](https://arxiv.org/html/2508.10914v1). Inspected: dataset, proposal/evaluation model, results, limitations and references.
- R15. Graham Todd et al. **Ludax: A GPU-Accelerated Domain Specific Language for Board Games.** [arXiv:2506.22609](https://arxiv.org/abs/2506.22609), first submitted in 2025; [v2 inspected](https://arxiv.org/html/2506.22609v2), March 25, 2026. Inspected: representation, compilation, supported game class and references. R12 cites the earlier 2025 version; this record deliberately distinguishes it from the newer text inspected.
- R21. Alexander Zook and Mark O. Riedl. **Automatic Game Design via Mechanic Generation.** AAAI 2014. [arXiv:1908.01420v1](https://arxiv.org/html/1908.01420v1), [DOI:10.1609/aaai.v28i1.8788](https://doi.org/10.1609/aaai.v28i1.8788). Inspected: representation, constraint-based generation, planning and demonstration domains. The arXiv deposit is 2019; the paper is from 2014. Promoted from the original R3/R7 citation lead.

## Metadata review recorded September 26, 2026

This later check did not reread the attributed paper findings or reproduce the experiments. Preserve its confirmed, unverified and contradicted distinctions. WS identifies metadata search results, not full-text inspection. FS, GR and GDR identify the supplied source collections.

| Source | Cited input | Check | Status | Qualification |
| --- | --- | --- | --- | --- |
| R1 Sorochan and Guzdial, arXiv 2212.03387, EXAG 2022 | FS, GDR | WS | Metadata only | Abstract confirms ten microRTS units; cause/effect vocabulary and 20-game evaluation not read |
| R3 GAVEL, arXiv 2407.09388, NeurIPS 2024 | FS, GDR | WS, clone the inspected revision | Metadata only | Repo citation matches; no root LICENSE file; "48 hours on RTX8000" not read |
| R7 Zook, Harrison and Riedl, arXiv 1908.01423 | GDR | WS | Metadata only | FDG 2015 version is titled "...Simulation-based Play Strategy Analysis" |
| R12 Collins et al., arXiv 2508.10914, CogSci 2025 | GDR | WS | Metadata only | Abstract says "over 450" games; draft's 484 not checked |
| R15 Ludax, arXiv 2506.22609 | GDR | WS, clone the inspected revision | Contradicted | Title is ["...Domain Specific Language for Board Games"](https://arxiv.org/abs/2506.22609); the draft says "Description Language". v2 date (2026-03-25) unchecked |
| R21 Zook and Riedl, AAAI 28(1) 2014, arXiv 1908.01420 | GDR | WS | Metadata only | |

Ludax title correction: the source draft used "GPU-Accelerated Description Language for Board Games". The later metadata check records "GPU-Accelerated Domain Specific Language for Board Games" as the title. That correction does not verify the attributed findings or the v2 date.

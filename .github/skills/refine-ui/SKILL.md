---
name: refine-ui
description: 'Refine approved UI through repeated real-browser screenshots and reference comparisons. Use when implementing or polishing layouts, matching supplied screenshots, or checking visual and interaction regressions.'
argument-hint: '<approved view or interaction and reference images>'
---

# Refine UI With Browser Evidence

## Boundaries

Refine explicitly approved behavior, not a new product specification. Read the
affected contracts and use [align-contract](../align-contract/SKILL.md) for
authorized implementation. Reference screenshots guide presentation; their
labels, features and data do not automatically approve new capabilities,
retention or refresh behavior.

## Iteration

1. Establish the affected view, approved interactions, existing shared owners
   and intended comparison state. Note the branch/revision and dirty changes.
   Read the existing implementation and browser tests before editing.
2. Save every owner-provided reference unchanged, with descriptive names, in the
   repository's ignored private reference directory. Track provenance without
   reproducing sensitive image contents. Verify copied files match originals;
   never publish or commit private references without explicit authorization.
3. Use the actual application templates, assets and production CSP in a real
   browser. Clearly label synthetic data; previews do not prove live backend
   behavior. Match viewport/content dimensions, theme, zoom, selected item/time,
   scroll position and hover/focus state as closely as practical.
4. Make a coherent, bounded set of changes. Then capture new screenshots and
   open them alongside the relevant references or prior captures. Repeat this
   comparison after each meaningful set of visual changes, not just at the end.
5. Compare panel geometry, spacing, typography, colors/contrast, controls,
   axes, markers, clipping, overlays and density. Check interaction meaning too:
   distinguish moving content from fixed controls, actual observations from
   inferred state, and a snapshot's latest time from live following.
6. Address the largest remaining mismatch, capture again and compare again.
   Keep useful before/after captures outside version control with clear names.
   Do not claim a numerical similarity percentage without a real measurement.
   Explain deliberate differences needed for accessibility or approved scope.
7. Exercise relevant desktop/compact layouts, both themes, keyboard and touch,
   overflow, zoom/grouping, hover/focus/Escape, empty/error states, no-JavaScript
   fallbacks and htmx reinitialization. Test changed geometry and behavior, not
   only selectors or screenshots; verify the production CSP remains effective.
   Use applicable browser projects and disclose unexecuted checks.
8. Review the changes, update directly related user guidance, and run focused
   tests followed by repository gates. Investigate failures rather than hiding
   them; separate environment/baseline failures from introduced defects.
9. Preserve requested final generated screenshots in session artifacts before
   another runner clears its output directory. Register only useful deliverables,
   never private references. Record actual verification and residual differences,
   then commit logical slices and follow the authorized delivery workflow.

Stop for owner input on consequential unapproved behavior. Do not iterate
indefinitely on subjective details or silently expand the feature to copy a
reference application's unrelated capabilities.

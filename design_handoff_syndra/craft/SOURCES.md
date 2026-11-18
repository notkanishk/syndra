# Sources for the craft

Every page the six research passes of 18 Nov 2026 used, grouped by theme, with one line on what it teaches. `CRAFT.md` is built from these.

**Marks.** No mark: the page was read (plain fetch, or in Chrome where it renders late). **Secondary:** a third party describing a product, not the maker. **UNVERIFIED:** not read, or seen only as a search summary; treat as a lead. **Failed:** tried, nothing usable; nothing is claimed from it.

**Corrections folded in (research 5).**
- Material 3's durations and curves are confirmed on its own site. Its easing system is now marked unmaintained in favour of springs.
- Apple's motion, feedback, layout and typography pages are confirmed. They give no millisecond or curve values.
- Apple never recommends tabular digits or a 16pt margin. Tabular figures are sourced from MDN.
- Bloomberg's amber for non-semantic information and its blue and red pair are confirmed; a black ground is not stated.
- The A List Apart piece on motion sensitivity is by Val Head, not Rachel Nabors.
- Still unverified: a loader's delay (150 to 300ms) and minimum time on screen (400ms); Tufte's data-ink ratio and small multiples in his own words.

---

## Motion and micro-interaction

| Source | What it teaches |
|---|---|
| https://emilkowal.ski/ui/you-dont-need-animations | Frequency decides whether to animate; keyboard actions never do; hover in daily tools is best still |
| https://emilkowal.ski/ui/7-practical-animation-tips | Press at 0.97; never from scale 0; origin-aware popovers; tooltips warm up once; blur hides a crossfade |
| https://emilkowal.ski/ui/great-animations | UI motion under 300ms; CSS over JS when the main thread is busy; Sonner's 400ms is a chosen exception |
| https://emilkowal.ski/ui/good-vs-great-animations | Easing matters most; ease-in-out for what is already on screen; no animation on functional data |
| https://emilkowal.ski/ui/building-a-toast-component | Transitions retarget and keyframes cannot; velocity-aware dismissal; pointer capture |
| https://emilkowal.ski/ui/the-magic-of-clip-path | A highlight that moves colour and shape together; long reveals are marketing, not product |
| https://rauno.me/craft/interaction-design | Menus appear at once and fade out; commit on release; origin says where a thing lives |
| https://rauno.me/craft | An index of prototypes by title. "Hold Enter to confirm" itself: UNVERIFIED |
| https://paco.me/ | Home page only. Failed in depth |
| https://benji.org/family-values | Persisting elements never duplicate; labels morph; delight in proportion to rarity |
| https://www.joshwcomeau.com/animation/css-transitions/ | Enter fast and leave slowly on hover; split the trigger from the effect so hover cannot flicker |
| https://www.joshwcomeau.com/animation/a-friendly-introduction-to-spring-physics/ | Springs for movement, not colour; most good springs do not bounce |
| https://www.joshwcomeau.com/animation/linear-timing-function/ | `linear()` approximates a spring for one-shots, and interrupts badly |
| https://www.joshwcomeau.com/animation/keyframe-animations/ | Fill modes: `forwards` to stay, `backwards` for a delayed start |
| https://www.joshwcomeau.com/blog/whimsical-animations | Novelty is the fuel and it runs out; tie effects to an action; keep one cohesive theme |
| https://developer.apple.com/videos/play/wwdc2018/803/ | A spring is damping and response; bounce only with momentum; project where a gesture would coast |
| https://developer.apple.com/design/human-interface-guidelines/motion | Avoid motion on frequent interactions; let people skip it; never the only carrier of meaning. No numbers |
| https://github.com/material-components/material-components-android/blob/master/docs/theming/Motion.md | Material's duration ladder (50ms steps) and its curves |
| https://m3.material.io/styles/motion/easing-and-duration/tokens-specs | The same tokens on Material's own site; easing now unmaintained in favour of springs |
| https://m3.material.io/styles/motion/easing-and-duration/applying-easing-and-duration | Enter 250 to 400ms, exit 200ms; a permanent exit ends at speed, a temporary one at rest |
| https://m3.material.io/styles/motion/overview/specs | Spatial motion may overshoot; colour and opacity never do |
| https://carbondesignsystem.com/elements/motion/overview/ | Productive against expressive motion; 70, 110, 150, 240, 400, 700ms |
| https://atlassian.design/foundations/motion | If it fires dozens of times a day, keep it under 150ms; exits faster than entrances |
| https://fluent2.microsoft.design/motion | Functional first; larger things take longer. No numbers |
| https://www.nngroup.com/articles/animation-duration/ | 100 to 400ms; never 500 and up; enter a little longer than exit |
| https://www.nngroup.com/articles/animation-usability/ | Motion is for attention, a change of state, or a relationship; do not displace text being read |
| https://www.nngroup.com/articles/microinteractions/ | A trigger and its feedback, near each other, with one of four purposes |
| https://www.oreilly.com/library/view/microinteractions/9781491945957/ | Saffer's trigger, rules, feedback, loops and modes. UNVERIFIED (book not read) |
| https://motion.dev/docs/react-layout-animations | Layout animation by transform; `scrollbar-gutter: stable` stops a jolt |
| https://motion.dev/docs/react-transitions | Spring parameters; a physics spring keeps velocity when redirected |
| https://number-flow.barvian.me/ | Digits that roll by direction, on tabular figures. Syndra does not roll numbers |
| https://developer.chrome.com/docs/web-platform/view-transitions/same-document | Named elements animate across states; one transition at a time; an enhancement |
| https://developer.mozilla.org/en-US/docs/Web/API/View_Transition_API/Using | Defaults, `match-element`, cross-document opt-in |
| https://web.dev/articles/animations-guide | Only transform and opacity stay off the main thread |
| https://web.dev/articles/prefers-reduced-motion | Reduce means remove non-essential movement; shorten instead of `animation: none` |
| https://alistapart.com/article/designing-safer-web-animation-for-motion-sensitivity/ | Val Head: triggers are distance, mismatched direction and zoom; colour and opacity are safe |
| https://www.smashingmagazine.com/2021/10/respecting-users-motion-preferences/ | Opt in with `no-preference`; keep a clear state indicator; one custom property per motion |
| https://stripe.com/en-cz/checkout | A marketing claim about care in animation; no specification. UNVERIFIED as evidence |
| https://www.awwwards.com/websites/sites_of_the_year/ | Names only; no jury prose |
| https://www.awwwards.com/sites/lando-norris | The scoring weights: design 40, usability 30, creativity 20, content 10 |

## Timing, perception and memory

| Source | What it teaches |
|---|---|
| https://www.nngroup.com/articles/response-times-3-important-limits/ | 0.1s, 1s and 10s: cause, flow, attention |
| https://lawsofux.com/doherty-threshold/ | Answer within 400ms and neither side waits. The 1982 paper itself: UNVERIFIED |
| https://www.nngroup.com/articles/progress-indicators/ | Nothing under 1s; a loop for 2 to 10s; a count past that ("3 of 50") |
| https://www.nngroup.com/articles/skeleton-screens/ | A skeleton shows real structure, for a whole page; a frame-only one says nothing |
| https://primer.style/product/ui-patterns/loading/ | Under 1s nothing; 1 to 3s a loop; 3 to 10s determinate; past 10s a background task |
| https://www.nngroup.com/articles/peak-end-rule/ | Memory keeps the peak and the end; failures are the vivid peaks |
| https://lawsofux.com/peak-end-rule/ | The same, as a design rule |
| https://lawsofux.com/zeigarnik-effect/ | Unfinished work stays in mind; a trusted inbox carries it instead |
| https://lawsofux.com/goal-gradient-effect/ | People speed up near a goal; the evidence in people is thin |
| https://www.nngroup.com/articles/aesthetic-usability-effect/ | Beauty raises perceived ease and forgives small faults only |
| https://lawsofux.com/aesthetic-usability-effect/ | Kurosu and Kashimura, 1995: 26 layouts, 252 people |
| https://www.nngroup.com/articles/short-term-memory-and-web-usability/ | About seven chunks, fading in seconds; compare side by side instead of asking to remember |
| https://lawsofux.com/millers-law/ | Seven is an average, not a cap |
| https://lawsofux.com/fittss-law/ | Size and distance set the time to hit; space between safe and dangerous is free safety |
| https://lawsofux.com/hicks-law/ | More choices take longer; mark one as recommended |
| https://lawsofux.com/jakobs-law/ | People expect what they already know; let them keep the old view for a while |
| https://lawsofux.com/teslers-law/ | Complexity moves between system and person; the system should carry it |
| https://lawsofux.com/law-of-proximity/ | Near things read as related |
| https://lawsofux.com/paradox-of-the-active-user/ | People start at once and do not read; teach in place |
| https://lawsofux.com/flow/ | Clear goals, immediate feedback, no added difficulty |
| https://www.csc2.ncsu.edu/faculty/healey/PP/ | The preattentive features; hue dominates shape; conjunctions need slow search |
| https://info3312.infosci.cornell.edu/slides/04-graphical-perception.html | Cleveland and McGill's ranking in summary. The 1984 paper: UNVERIFIED |

## Space, type and density

| Source | What it teaches |
|---|---|
| https://carbondesignsystem.com/elements/spacing/overview/ | A spacing scale in three bands, used by meaning |
| https://carbondesignsystem.com/elements/2x-grid/overview/ | Everything a multiple of 8; a 16-column grid |
| https://primer.style/foundations/primitives/size | A finer scale; control heights 28, 32, 40 |
| https://carbondesignsystem.com/elements/typography/type-sets/ | Productive type: 14/20 body, 12/16 label; fluid type never inside a container |
| https://vercel.com/geist/typography | Styles named by role; mono as the voice of a value |
| https://practicaltypography.com/point-size.html | Reading text 15 to 25px on the web |
| https://practicaltypography.com/line-spacing.html | Line spacing 120 to 145% |
| https://practicaltypography.com/line-length.html | 45 to 90 characters a line |
| https://practicaltypography.com/summary-of-key-rules.html | Capitals for less than a line, with 5 to 12% tracking |
| https://practicaltypography.com/tables.html | Take every border off; add back what helps; padding does the rest |
| https://developer.mozilla.org/en-US/docs/Web/CSS/font-variant-numeric | `tabular-nums`, `lining-nums`, `slashed-zero`, if the font carries them |
| https://developer.apple.com/design/human-interface-guidelines/typography | Mac text styles (body 13/16, title 22/26); floors of 10 to 11; no light weights |
| https://developer.apple.com/design/human-interface-guidelines/layout | Alignment reads as relation; same function at every width. No 16pt margin, no tabular digits |
| https://m3.material.io/styles/typography/type-scale-tokens | A 1.125 ratio on a 14 base; one emphasis mechanism. The numeric table: UNVERIFIED |
| https://m3.material.io/foundations/layout/grids-spacing/density | Density is the person's choice, changes padding only, never dialogs |
| https://carbondesignsystem.com/components/data-table/style/ | Row heights 24 to 64; 16 between columns; the header matches the rows |
| https://www.nngroup.com/articles/data-tables/ | Sticky header; a readable first column; one row-tracking aid |
| https://www.nngroup.com/articles/gestalt-proximity/ | A heading sits nearer its own section; check groupings at every width |
| https://www.nngroup.com/articles/visual-hierarchy-ux-definition/ | Three sizes, two large elements, the squint test |
| https://www.nngroup.com/articles/layer-cake-pattern-scanning/ | Headings as scan targets: the most effective way people scan |
| https://www.nngroup.com/articles/text-scanning-patterns-eyetracking/ | The four scanning patterns |
| https://www.nngroup.com/articles/progressive-disclosure/ | Frequent things first; two levels at most; a labelled way to the rest |
| https://www.nngroup.com/articles/empty-state-interface-design/ | A blank cannot be told from loading or failure; say which, and give a way on |
| https://www.nngroup.com/articles/f-shaped-pattern-reading-web-content-discovered/ | Unbroken text gets an F-shaped read |
| https://carbondesignsystem.com/data-visualization/dashboards/ | Rank by importance; its "20 percent" figure is Carbon's claim, unchecked |
| https://linear.app/now/how-we-redesigned-the-linear-ui | Alignment felt "after a few minutes"; a theme from three inputs; a display cut for headings |
| https://linear.app/now/behind-the-latest-design-refresh | A dimmer sidebar, softer dividers, a warmer grey |
| https://www.bloomberg.com/company/stories/how-bloomberg-terminal-ux-designers-conceal-complexity/ | Change in small steps; wider windows show more rows; never change look and behaviour together |
| https://www.bloomberg.com/company/stories/designing-the-terminal-for-color-accessibility/ | Thin red and green lines fail first; amber for what has no meaning; a blue and red pair |
| https://stripe.com/blog/accessible-color-systems | A palette built in a perceptual space; "five levels apart" passes for small text |
| https://www.edwardtufte.com/notebook/sparkline-theory-and-practice-edward-tufte/ | Word-sized graphics; a grey normal band; the last point tied to the number; one scale when comparing |
| https://www.edwardtufte.com/book/the-visual-display-of-quantitative-information/ | Names data-ink and small multiples only. Their definitions: UNVERIFIED |
| https://attio.com/blog | Nothing on design. Failed |
| https://attio.com/engineering/blog | Nothing on the table UI |
| https://attio.com/engineering/blog/react-data-list-building-virtualized-uis-declaratively | Pending and error per section, not for the whole list |
| https://baymard.com/blog/inline-form-validation | Paywalled. Failed |

## Consoles: plan, safety, explanation, state

| Source | What it teaches |
|---|---|
| https://developer.hashicorp.com/terraform/cli/commands/plan | A closed set of verbs, one summary line, the reason beside the surprise |
| https://planetscale.com/docs/vitess/schema-changes/deploy-requests | A change as an object; approval bound to content; an honest 30-minute undo |
| https://tailscale.com/kb/1337/acl-syntax | A policy carries its own must and must-not tests. Its visual editor: UNVERIFIED |
| https://vercel.com/docs/instant-rollback | A confirm that lists what changes and what stays; a standing state kept visible |
| https://www.nngroup.com/articles/confirmation-dialog/ | Undo beats confirmation; say the consequence; name buttons by outcome |
| https://alistapart.com/article/neveruseawarning/ | Raskin: a repeated warning is dismissed by reflex; allow reversal instead |
| https://www.nngroup.com/articles/user-control-and-freedom/ | Undo must be findable; a fleeting one is a trap |
| https://www.nngroup.com/articles/ten-usability-heuristics/ | State before consequence; prevent the costly errors first |
| https://www.nngroup.com/articles/visibility-system-status/ | Never change state without saying so |
| https://www.nngroup.com/articles/error-message-guidelines/ | Near the cause, plain, specific, a next step, input kept, no jokes |
| https://docs.cloud.google.com/policy-intelligence/docs/troubleshoot-access | Why someone has access, per layer, with the deciding one flagged and Unknown shown |
| https://developers.cloudflare.com/cloudflare-one/policies/access/policy-management/ | Test a policy against real people: a share, then the names |
| https://developers.cloudflare.com/cloudflare-one/policies/access/ | No tester on this page; superseded by the one above |
| https://workos.com/docs/directory-sync/understanding-events | State is reconciled; events are hints |
| https://workos.com/docs/admin-portal.md | Status, a test action and recent evidence on one screen; a danger zone that says what it leaves alone |
| https://primer.style/product/ui-patterns/degraded-experiences/ | Degrade locally; hide a count that cannot be read; never an empty state where data should be |
| https://primer.style/product/ui-patterns/saving/ | Explicit save for forms, instant for switches; a failed save keeps the input |
| https://carbondesignsystem.com/patterns/notification-pattern/ | A ladder from inline to modal; nothing critical times out |
| https://carbondesignsystem.com/patterns/status-indicator-pattern/ | Two cues per status; a small set; the highest wins |
| https://carbondesignsystem.com/patterns/dialog-pattern/ | No dialog when the person needs information outside it; no nesting |
| https://carbondesignsystem.com/patterns/empty-states-pattern/ | Four kinds of empty; one action; never a dead end |
| https://atlassian.design/foundations/content/designing-messages/ | A warning comes before, an error after |
| https://atlassian.design/components/empty-state/usage | Tone follows the cause: a beginning, a finish, a miss |
| https://atlassian.design/components/modal-dialog/usage | Focus order; return focus to the trigger; the button echoes the title |
| https://shopify.dev/docs/api/app-home/web-components/overlays/modal | Polaris: specific verbs; the consequence in the body; hold the dialog until success |
| https://goteleport.com/docs/identity-governance/access-requests/ | Elevation that ends by default |
| https://docs.github.com/en/organizations/managing-user-access-to-your-organizations-repositories/managing-repository-roles/repository-roles-for-an-organization | Each role in one sentence about who it is for; the exceptions written down |
| https://clerk.com/docs/guides/organizations/roles-permissions | A role cannot be deleted while held; the creator's role is protected |
| https://docs.sentry.io/product/issues/states-triage/ | Silence that reopens itself when things change |
| https://support.1password.com/watchtower/ | Problems grouped by cause, each with one fix; empty groups hidden |
| https://docs.stripe.com/development/dashboard/request-logs | Filter a log by the one thing, and by who acted |
| https://docs.stripe.com/test-mode | A test place that is isolated and says so loudly |
| https://stripe.com/us/blog/workbench-a-new-way-to-debug-monitor-and-grow-your-stripe-integration | An inspector beside the object; errors rolled up by cause; a link to any view |
| https://docs.stripe.com/workbench/overview | The same. UNVERIFIED (search summary) |

## Calm, quality and ethics

| Source | What it teaches |
|---|---|
| https://calmtech.com/ | The smallest possible attention; use the periphery; keep working when it fails. Weiser and Brown's essay: UNVERIFIED |
| https://www.vitsoe.com/us/about/good-design | Rams: honest, unobtrusive, thorough to the last detail |
| https://linear.app/blog/why-is-quality-so-rare | Quality as the deciding criterion; its business claims are Linear's own |
| https://www.figma.com/blog/karri-saarinens-10-rules-for-crafting-products-that-stand-out/ | The spec is the baseline; reduce scope to raise quality; to delight you must surprise |
| https://linear.app/changelog | Polish as position, focus and scroll that survive every change |
| https://www.deceptive.design/ | The eighteen deceptive patterns, as a list of things never to build |
| https://www.deceptive.design/types/nagging | Asking again and again for what does not serve the person |
| https://www.deceptive.design/types/confirmshaming | Guilt on the decline button |
| https://www.humanetech.com/ | A mission statement only; no position quoted |

## Delight and character

| Source | What it teaches |
|---|---|
| https://www.nngroup.com/articles/theory-user-delight/ | Surface against deep delight; Walter's pyramid; forced humour reads as insincere |
| https://www.nngroup.com/articles/pillars-user-delight/ | Visceral, behavioural, reflective; no playfulness when someone is under stress |
| https://styleguide.mailchimp.com/voice-and-tone/ | One voice, a tone that moves; a mascot that winks and never speaks |
| https://slack.design/articles/thevoiceofthebrand-5principles/ | Memorable and respectful; playfulness that does not distract |
| https://conffab.com/presentation/many-hands-one-voice/ | Slack's humour sits at the empty and end stages. Secondary |
| https://bradfrost.com/blog/post/clarity-conf-being-human-being-slack/ | The same, from a conference note. Secondary |
| https://blakecrosley.com/blog/design-study-things | Things 3: plain nouns, colour only for meaning, a half-second checkbox. Secondary |
| https://www.macstories.net/reviews/things-3-beauty-and-delight-in-a-task-manager/ | Things 3's haptics and card motion, from a review. Secondary |
| https://blakecrosley.com/guides/design/arc | Arc: a space with its own colour. Secondary; the sample CSS is the author's |
| https://clipcontent.substack.com/p/the-playful-design-details-of-the | Playdate: care in the edges, a first boot that teaches, celebration for arrivals. Secondary |
| https://blog.duolingo.com/world-character-visemes | Characters that react to your outcome; believability judged in motion |
| https://notbor.ing/words/the-most-satisfying-checkbox | Feedback begins with the press; reverse cleanly; test with sound off |
| https://www.macrumors.com/guide/ios-15-weather-app | Apple Weather: the background is the conditions. Secondary |
| https://9to5mac.com/2021/09/24/ios-15-weather-app-hands-on/ | The same. Secondary |
| https://www.bram.us/2021/10/13/how-to-create-the-stripe-website-gradient-effect | How Stripe's moving gradient is built; a reason to keep Syndra's still |
| https://stripe.com/blog/globe | Stripe's globe essay. UNVERIFIED (search result only) |
| https://stripepress.substack.com/p/behind-the-cover-scaling-people-by | One material gesture per surface. Secondary |
| https://styles.refero.design/style/54f257e9-1d6b-4410-b94a-1dfe648ecc87 | A description of Stripe Press's site. UNVERIFIED |
| https://manual.raycast.com/v1/deeplinks | Confetti as something the person summons. Secondary (via search) |
| https://raycast-frontend-5xlduik3r-raycastapp.vercel.app/community-stories/sania-saleh | A designer's account of that command. Secondary |
| https://www.engadget.com/interview-with-nothing-ceo-carl-pei-thinks-everyone-elses-smartphones-are-boring-090011028.html | Nothing: one signal, one meaning, in sound and light. Secondary |
| https://airbnb.design/introducing-lottie | Lottie's origin. Failed to load; UNVERIFIED |
| https://epicreact.dev/modules/epic-react-expert-interviews/animations-and-interactions-with-josh-comeau | Delight can be used to coax; a caution. Secondary |
| https://developer.apple.com/design/human-interface-guidelines/feedback | Match the weight of the message; status near its subject; confirm only what is significant |

## Not read, and named so nobody cites them from here

Rachel Nabors on reduced motion (not found) · Paco Coursey's essays (home page only) · Tufte on data-ink and small multiples (book page names them only) · Baymard (paywalled) · Atlassian's destructive-action guidance · Awwwards jury commentary · Material 3's numeric type table · Kahneman's, Norman's, Walter's, Csikszentmihalyi's, Saffer's and Raskin's books · Doherty and Thadani (1982) · Cleveland and McGill (1984) · Tractinsky (2000) · Gloria Mark on interruption · Weiser and Brown (1996) · Okta, Auth0, Zitadel, Fly.io, Supabase, Railway, Render, Grafana, Datadog, Retool, Height and Google Workspace Admin as products.

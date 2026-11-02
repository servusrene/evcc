<template>
	<div class="message-log d-flex flex-column" :class="{ 'message-log--fill': fill }">
		<div ref="list" class="message-list border rounded" @scroll="onScroll">
			<ul class="list-unstyled mb-0">
				<li v-for="entry in entries" :key="entry.key" class="entry">
					<button
						type="button"
						class="entry-header btn btn-link text-reset text-decoration-none d-flex align-items-baseline gap-2 w-100 px-2 py-1"
						:aria-expanded="isExpanded(entry.key)"
						@click="toggle(entry.key)"
					>
						<span class="time text-muted">{{ entry.time }}</span>
						<span
							class="direction"
							:class="`direction--${entry.direction}`"
							:title="$t(`config.ocppmaintenance.log.${entry.direction}`)"
							:aria-label="$t(`config.ocppmaintenance.log.${entry.direction}`)"
							>{{ arrow(entry.direction) }}</span
						>
						<span class="label text-truncate" :class="{ 'text-danger': entry.error }">
							{{ entry.label }}
						</span>
					</button>
					<pre v-if="isExpanded(entry.key)" class="payload mx-2 mb-2 p-2 rounded">{{
						entry.payload
					}}</pre>
				</li>
			</ul>
		</div>
		<div class="mt-2">
			<ul
				class="legend list-unstyled d-flex flex-wrap gap-2 mb-0"
				data-testid="ocppmaintenance-log-legend"
			>
				<li
					v-for="direction in directions"
					:key="direction"
					class="legend-item d-flex align-items-center gap-1 border rounded-pill px-2 text-nowrap"
				>
					<span class="direction" :class="`direction--${direction}`">{{
						arrow(direction)
					}}</span>
					{{ $t(`config.ocppmaintenance.log.${direction}Legend`) }}
				</li>
			</ul>
			<p v-if="help" class="legend-help mt-1 mb-0">{{ help }}</p>
		</div>
	</div>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";

export interface OcppMessage {
	time: string;
	direction: "send" | "recv";
	frame: string;
}

interface Entry {
	key: string;
	time: string;
	direction: OcppMessage["direction"];
	label: string;
	payload: string;
	error: boolean;
}

const CALL = 2;
const CALL_RESULT = 3;
const CALL_ERROR = 4;
// distance in px below which the list counts as scrolled to the end
const FOLLOW_THRESHOLD = 20;

export default defineComponent({
	name: "OcppMessageLog",
	props: {
		messages: { type: Array as PropType<OcppMessage[]>, default: () => [] },
		expandAll: { type: Boolean, default: false },
		// on large screens take the remaining height of a flex column instead of a fixed one
		fill: { type: Boolean, default: false },
		help: { type: String, default: "" },
	},
	data() {
		return {
			directions: ["send", "recv"] as OcppMessage["direction"][],
			shown: [] as OcppMessage[],
			follow: true,
			// entries toggled against the expandAll default
			toggled: {} as Record<string, boolean>,
		};
	},
	computed: {
		entries(): Entry[] {
			// results only carry the message id, resolve the action from the matching call
			const actions = new Map<string, string>();
			return this.shown.map((msg, index) => {
				const entry: Entry = {
					key: `${msg.time}-${index}-${msg.direction}`,
					time: new Date(msg.time).toLocaleTimeString(this.$i18n?.locale),
					direction: msg.direction,
					label: msg.frame,
					payload: msg.frame,
					error: false,
				};

				let frame: unknown;
				try {
					frame = JSON.parse(msg.frame);
				} catch {
					return entry;
				}
				if (!Array.isArray(frame)) return entry;

				const [type, id] = frame;
				entry.key = `${msg.direction}-${type}-${id}-${index}`;
				if (type === CALL) {
					actions.set(id, frame[2]);
					entry.label = frame[2];
					entry.payload = JSON.stringify(frame[3], null, 2);
				} else if (type === CALL_RESULT) {
					const action = actions.get(id);
					entry.label = action
						? this.$t("config.ocppmaintenance.log.result", { action })
						: this.$t("config.ocppmaintenance.log.resultUnknown");
					entry.payload = JSON.stringify(frame[2], null, 2);
				} else if (type === CALL_ERROR) {
					entry.error = true;
					entry.label = this.$t("config.ocppmaintenance.log.error", {
						code: frame[2],
					});
					entry.payload = JSON.stringify(
						{ code: frame[2], description: frame[3], details: frame[4] },
						null,
						2
					);
				}
				return entry;
			});
		},
	},
	watch: {
		expandAll() {
			this.toggled = {};
		},
		messages: {
			immediate: true,
			handler(messages: OcppMessage[]) {
				// freeze the list while the user reads older messages
				if (!this.follow) return;
				this.shown = messages;
				this.$nextTick(this.scrollToEnd);
			},
		},
	},
	methods: {
		arrow(direction: OcppMessage["direction"]): string {
			return direction === "send" ? "→" : "←";
		},
		isExpanded(key: string): boolean {
			return this.expandAll !== !!this.toggled[key];
		},
		toggle(key: string) {
			this.toggled[key] = !this.toggled[key];
		},
		onScroll() {
			const el = this.$refs["list"] as HTMLElement | undefined;
			if (!el) return;
			const follow = el.scrollHeight - el.scrollTop - el.clientHeight < FOLLOW_THRESHOLD;
			if (follow && !this.follow) {
				this.shown = this.messages;
			}
			this.follow = follow;
		},
		scrollToEnd() {
			const el = this.$refs["list"] as HTMLElement | undefined;
			if (el) el.scrollTop = el.scrollHeight;
		},
	},
});
</script>

<style scoped>
@import "../../../css/breakpoints.css";

.message-log {
	--bs-border-color: var(--bs-gray-light);
	--log-send: var(--evcc-green);
	--log-recv: var(--evcc-temp-cold);
	font-size: 0.875rem;
}

:global(.dark) .message-log {
	--log-recv: #6cb4ee;
}

.message-list {
	max-height: 20rem;
	overflow-y: auto;
	overscroll-behavior: contain;
}

@media (--lg-and-up) {
	.message-log--fill {
		flex: 1 1 0;
		min-height: max(20rem, 60vh);
	}

	.message-log--fill .message-list {
		flex: 1 1 0;
		max-height: none;
	}
}

.entry + .entry {
	border-top: 1px solid var(--bs-border-color);
}

.entry-header {
	font-size: inherit;
	text-align: left;
	border-radius: 0;
}

.entry-header:hover {
	background-color: var(--evcc-box-border);
}

.time {
	font-variant-numeric: tabular-nums;
	flex-shrink: 0;
}

.direction {
	flex-shrink: 0;
	width: 1em;
	text-align: center;
	font-weight: bold;
}

.direction--send {
	color: var(--log-send);
}

.direction--recv {
	color: var(--log-recv);
}

.legend-item,
.legend-help {
	font-size: 0.75rem;
	color: var(--bs-gray-medium);
}

.label {
	min-width: 0;
}

.payload {
	font-size: 0.75rem;
	white-space: pre-wrap;
	word-break: break-word;
	background-color: var(--evcc-background);
	color: var(--evcc-default-text);
}
</style>

<template>
	<GenericModal
		id="ocppMaintenanceModal"
		config-modal-name="ocppmaintenance"
		:title="$t('config.ocppmaintenance.title')"
		data-testid="ocppmaintenance-modal"
		size="xl"
		@open="open"
		@closed="close"
	>
		<p class="text-muted mb-3">
			{{ $t("config.ocppmaintenance.description") }}
		</p>

		<div v-if="loadError" class="alert alert-danger" role="alert">
			{{ loadError }}
		</div>

		<div
			v-if="pending"
			class="alert mb-3 d-flex flex-column flex-sm-row align-items-sm-center gap-3"
			:class="pending.type === 'hard' ? 'alert-danger' : 'alert-warning'"
			role="alert"
			data-testid="ocppmaintenance-confirm"
		>
			<p class="mb-0 flex-grow-1">{{ confirmText }}</p>
			<div class="d-flex justify-content-end gap-2 flex-shrink-0">
				<button type="button" class="btn btn-sm btn-link text-muted" @click="cancel">
					{{ $t("config.general.cancel") }}
				</button>
				<button
					type="button"
					class="btn btn-sm"
					:class="pending.type === 'hard' ? 'btn-danger' : 'btn-primary'"
					:disabled="busy"
					@click="confirm"
				>
					{{ $t(`config.ocppmaintenance.${pending.type}.confirm`) }}
				</button>
			</div>
		</div>

		<div
			v-if="result"
			class="alert mb-3 d-flex align-items-start gap-2"
			:class="result.success ? 'alert-success' : 'alert-danger'"
			role="status"
			data-testid="ocppmaintenance-result"
		>
			<span class="flex-grow-1">{{ result.message }}</span>
			<button
				v-if="!result.success"
				type="button"
				class="btn-close"
				:aria-label="$t('config.general.close')"
				@click="clearResult"
			></button>
		</div>

		<div v-if="station" class="row gx-5 gy-2 gy-lg-0">
			<div class="col-lg-5 col-xl-4">
				<h6 class="mt-0 mb-3">{{ $t("config.ocppmaintenance.station") }}</h6>
				<dl class="station-info mb-4" data-testid="ocppmaintenance-info">
					<dt>{{ $t("config.ocppmaintenance.stationId") }}</dt>
					<dd class="text-break">
						<code>{{ station.id }}</code>
					</dd>
					<dt>{{ $t("config.ocppmaintenance.connection") }}</dt>
					<dd :class="connected ? 'text-success' : 'text-danger'">
						{{ $t(`config.ocpp.status.${station.status}`) }}
					</dd>
					<template v-for="field in infoFields" :key="field.key">
						<dt>{{ $t(`config.ocppmaintenance.${field.key}`) }}</dt>
						<dd class="text-break">{{ field.value }}</dd>
					</template>
				</dl>

				<h6 class="mb-3">{{ $t("config.ocppmaintenance.connectors") }}</h6>
				<p v-if="station.connectors.length === 0" class="text-muted">
					{{ $t("config.ocppmaintenance.noConnectors") }}
				</p>
				<ul class="list-unstyled d-flex flex-column gap-3 mb-4">
					<li
						v-for="connector in station.connectors"
						:key="connector.id"
						class="connector border rounded p-3"
						data-testid="ocppmaintenance-connector"
					>
						<div class="d-flex justify-content-between gap-2 mb-2">
							<strong class="text-nowrap">
								{{ $t("config.ocppmaintenance.connector", { id: connector.id }) }}
							</strong>
							<span :class="connector.errorCode ? 'text-danger' : 'text-muted'">
								{{ connectorStatus(connector) }}
							</span>
						</div>
						<p class="small text-muted mb-3">
							{{ transactionLabel(connector) }}
						</p>
						<div class="d-flex flex-wrap gap-2">
							<button
								type="button"
								class="btn btn-sm btn-outline-secondary"
								:disabled="!connected || !connector.transactionId || busy"
								@click="ask({ type: 'stop', connector: connector.id })"
							>
								{{ $t("config.ocppmaintenance.stop.button") }}
							</button>
							<button
								type="button"
								class="btn btn-sm btn-outline-secondary"
								:disabled="!connected || busy"
								@click="ask({ type: 'unlock', connector: connector.id })"
							>
								{{ $t("config.ocppmaintenance.unlock.button") }}
							</button>
						</div>
					</li>
				</ul>

				<h6 class="mb-3">{{ $t("config.ocppmaintenance.restart") }}</h6>
				<div class="d-flex flex-wrap gap-2">
					<button
						type="button"
						class="btn btn-sm btn-outline-secondary"
						:disabled="!connected || busy"
						@click="ask({ type: 'soft' })"
					>
						{{ $t("config.ocppmaintenance.soft.button") }}
					</button>
					<button
						type="button"
						class="btn btn-sm btn-outline-danger"
						:disabled="!connected || busy"
						@click="ask({ type: 'hard' })"
					>
						{{ $t("config.ocppmaintenance.hard.button") }}
					</button>
				</div>

				<button
					type="button"
					class="btn btn-sm btn-link text-muted px-0 mt-2 d-lg-none"
					:aria-expanded="showLog"
					@click="toggleLog"
				>
					{{ $t(`config.ocppmaintenance.log.${showLog ? "hide" : "show"}`) }}
				</button>
			</div>

			<div
				class="col-lg-7 col-xl-8 flex-column"
				:class="showLog ? 'd-flex' : 'd-none d-lg-flex'"
				data-testid="ocppmaintenance-log"
			>
				<div
					class="d-flex justify-content-between align-items-baseline gap-2 mt-4 mt-lg-0 mb-3"
				>
					<h6 class="my-0">{{ $t("config.ocppmaintenance.log.title") }}</h6>
					<button
						v-if="messages.length > 0"
						type="button"
						class="btn btn-sm btn-link text-muted p-0"
						@click="expandAll = !expandAll"
					>
						{{ expandLabel }}
					</button>
				</div>
				<p v-if="messages.length === 0" class="small text-muted">
					{{ $t("config.ocppmaintenance.log.empty") }}
				</p>
				<OcppMessageLog
					v-else
					:messages="messages"
					:expand-all="expandAll"
					:help="$t('config.ocppmaintenance.log.help')"
					fill
				/>
			</div>
		</div>

		<div class="d-flex justify-content-end mt-4">
			<button type="button" class="btn btn-outline-primary px-4" data-bs-dismiss="modal">
				{{ $t("config.general.close") }}
			</button>
		</div>
	</GenericModal>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import type { AxiosResponse } from "axios";
import GenericModal from "../Helper/GenericModal.vue";
import OcppMessageLog, { type OcppMessage } from "./OcppMessageLog.vue";
import { getModal } from "@/configModal";
import api from "@/api";
import { OCPP_STATION_STATUS } from "@/types/evcc";

export interface OcppConnectorInfo {
	id: number;
	status?: string;
	errorCode?: string;
	info?: string;
	transactionId?: number;
	idTag?: string;
}

export interface OcppStationInfo {
	id: string;
	status: OCPP_STATION_STATUS;
	vendor?: string;
	model?: string;
	serialNumber?: string;
	firmwareVersion?: string;
	connectors: OcppConnectorInfo[];
}

type ActionType = "stop" | "unlock" | "soft" | "hard";

interface Action {
	type: ActionType;
	connector?: number;
}

const REFRESH_INTERVAL = 5000;
const RESULT_DURATION = 5000;
const INFO_KEYS = ["vendor", "model", "serialNumber", "firmwareVersion"] as const;

export default defineComponent({
	name: "OcppMaintenanceModal",
	components: { GenericModal, OcppMessageLog },
	data() {
		return {
			station: null as OcppStationInfo | null,
			loadError: "",
			pending: null as Action | null,
			busy: false,
			result: null as { success: boolean; message: string } | null,
			timer: undefined as ReturnType<typeof setInterval> | undefined,
			showLog: false,
			expandAll: true,
			messages: [] as OcppMessage[],
			resultTimer: undefined as ReturnType<typeof setTimeout> | undefined,
		};
	},
	computed: {
		// station id the modal is showing, carried via the config modal stack
		stationId(): string {
			return getModal("ocppmaintenance")?.station || "";
		},
		stationPath(): string {
			return `/config/ocpp/stations/${encodeURIComponent(this.stationId)}`;
		},
		connected(): boolean {
			return this.station?.status === OCPP_STATION_STATUS.CONNECTED;
		},
		infoFields(): { key: string; value: string }[] {
			const station = this.station;
			if (!station) return [];
			return INFO_KEYS.filter((key) => station[key]).map((key) => ({
				key,
				value: station[key] as string,
			}));
		},
		expandLabel(): string {
			return this.$t(
				`config.ocppmaintenance.log.${this.expandAll ? "collapseAll" : "expandAll"}`
			);
		},
		confirmText(): string {
			if (!this.pending) return "";
			return this.$t(`config.ocppmaintenance.${this.pending.type}.confirmText`, {
				id: this.pending.connector,
			});
		},
	},
	unmounted() {
		this.stopRefresh();
		clearTimeout(this.resultTimer);
	},
	methods: {
		open() {
			this.station = null;
			this.loadError = "";
			this.pending = null;
			this.clearResult();
			this.showLog = false;
			this.expandAll = true;
			this.messages = [];
			this.refresh();
			this.stopRefresh();
			this.timer = setInterval(this.refresh, REFRESH_INTERVAL);
		},
		close() {
			this.stopRefresh();
			this.clearResult();
		},
		stopRefresh() {
			if (this.timer) {
				clearInterval(this.timer);
				this.timer = undefined;
			}
		},
		async load() {
			if (!this.stationId) return;
			const res = await this.request("get", this.stationPath);
			if (res.status === 200) {
				this.station = res.data as OcppStationInfo;
				this.loadError = "";
			} else {
				this.loadError = this.errorMessage(res);
			}
		},
		async loadLog() {
			if (!this.stationId) return;
			const res = await this.request("get", `${this.stationPath}/log`);
			if (res.status !== 200) return;
			this.messages = Array.isArray(res.data) ? (res.data as OcppMessage[]) : [];
		},
		toggleLog() {
			this.showLog = !this.showLog;
		},
		clearResult() {
			clearTimeout(this.resultTimer);
			this.result = null;
		},
		refresh() {
			this.load();
			this.loadLog();
		},
		// handle every status locally, the global error toast would duplicate the inline result
		request(method: "get" | "post", url: string): Promise<AxiosResponse> {
			return api.request({ method, url, validateStatus: () => true }).catch((error) => ({
				status: 0,
				data: { error: error.message },
			})) as Promise<AxiosResponse>;
		},
		errorMessage(res: AxiosResponse): string {
			return res.data?.error || this.$t("config.ocppmaintenance.failed");
		},
		connectorStatus(connector: OcppConnectorInfo): string {
			const status = connector.status || this.$t("config.ocpp.status.unknown");
			if (!connector.errorCode) return status;
			const details = [connector.errorCode, connector.info].filter(Boolean).join(": ");
			return `${status} (${details})`;
		},
		transactionLabel(connector: OcppConnectorInfo): string {
			if (!connector.transactionId) {
				return this.$t("config.ocppmaintenance.noTransaction");
			}
			return this.$t("config.ocppmaintenance.transaction", {
				id: connector.transactionId,
				idTag: connector.idTag || "-",
			});
		},
		ask(action: Action) {
			this.clearResult();
			this.pending = action;
		},
		cancel() {
			this.pending = null;
		},
		actionPath(action: Action): string {
			switch (action.type) {
				case "soft":
				case "hard":
					return `${this.stationPath}/reset/${action.type}`;
				default:
					return `${this.stationPath}/connectors/${action.connector}/${action.type}`;
			}
		},
		async confirm() {
			const action = this.pending;
			if (!action) return;

			this.busy = true;
			const res = await this.request("post", this.actionPath(action));
			this.busy = false;
			this.pending = null;

			const success = res.status === 200;
			this.result = {
				success,
				message: success
					? this.$t(`config.ocppmaintenance.${action.type}.success`)
					: this.errorMessage(res),
			};
			// errors stay until dismissed, the user must not miss a rejection
			if (success) {
				this.resultTimer = setTimeout(this.clearResult, RESULT_DURATION);
			}

			await this.refresh();
		},
	},
});
</script>

<style>
@import "../../../css/breakpoints.css";

/* the log needs more room than modal-xl offers below the xl breakpoint */
@media (--lg-and-up) {
	#ocppMaintenanceModal .modal-dialog {
		max-width: min(1600px, calc(100vw - 4rem));
	}
}
</style>

<style scoped>
.station-info {
	display: grid;
	grid-template-columns: max-content minmax(0, 1fr);
	column-gap: 1.5rem;
	row-gap: 0.25rem;
}

.station-info dd {
	margin-bottom: 0;
}

.connector {
	--bs-border-color: var(--bs-gray-light);
}
</style>

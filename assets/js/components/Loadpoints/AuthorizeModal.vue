<template>
	<GenericModal
		id="authorizeModal"
		ref="modal"
		:title="$t('main.authorizeModal.title')"
		data-testid="authorize-modal"
		size="sm"
		@closed="reset"
	>
		<form @submit.prevent="submit">
			<label :for="inputId" class="form-label">
				{{ $t("main.authorizeModal.pin") }}
			</label>
			<input
				:id="inputId"
				v-model="pin"
				type="password"
				class="form-control text-center fs-4"
				inputmode="numeric"
				pattern="[0-9]{4}"
				maxlength="4"
				autocomplete="off"
				required
			/>
			<p v-if="errorMessage" class="text-danger mt-3 mb-0" role="alert">
				{{ errorMessage }}
			</p>
			<div class="mt-4 d-flex justify-content-between gap-2">
				<button type="button" class="btn btn-link text-muted" data-bs-dismiss="modal">
					{{ $t("config.general.cancel") }}
				</button>
				<button type="submit" class="btn btn-primary" :disabled="submitDisabled">
					<span
						v-if="loading"
						class="spinner-border spinner-border-sm me-2"
						role="status"
						aria-hidden="true"
					></span>
					{{ $t("main.authorizeModal.submit") }}
				</button>
			</div>
		</form>
	</GenericModal>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import api, { allowClientError } from "@/api";
import GenericModal from "../Helper/GenericModal.vue";

const PIN_LENGTH = 4;

export default defineComponent({
	name: "AuthorizeModal",
	components: { GenericModal },
	data() {
		return {
			id: "",
			pin: "",
			loading: false,
			errorMessage: "",
		};
	},
	computed: {
		inputId(): string {
			return `authorizePin_${this.id}`;
		},
		submitDisabled(): boolean {
			return this.loading || this.pin.length !== PIN_LENGTH;
		},
	},
	methods: {
		open(loadpointId: string) {
			this.reset();
			this.id = loadpointId;
			(this.$refs["modal"] as InstanceType<typeof GenericModal> | undefined)?.open();
		},
		close() {
			(this.$refs["modal"] as InstanceType<typeof GenericModal> | undefined)?.close();
		},
		reset() {
			this.pin = "";
			this.loading = false;
			this.errorMessage = "";
		},
		async submit() {
			if (this.submitDisabled) return;
			this.loading = true;
			this.errorMessage = "";
			try {
				const res = await api.post(
					`loadpoints/${this.id}/authorize`,
					{ pin: this.pin },
					allowClientError
				);
				if (res.status === 200) {
					this.close();
					return;
				}
				this.pin = "";
				this.errorMessage = this.errorFor(res.status, res.data?.error);
			} finally {
				this.loading = false;
			}
		},
		errorFor(status: number, fallback?: string): string {
			switch (status) {
				case 403:
					return this.$t("main.authorizeModal.invalidPin");
				case 429:
					return this.$t("main.authorizeModal.locked");
				case 409:
					return this.$t("main.authorizeModal.notWaiting");
				default:
					return fallback || this.$t("main.authorizeModal.failed");
			}
		},
	},
});
</script>

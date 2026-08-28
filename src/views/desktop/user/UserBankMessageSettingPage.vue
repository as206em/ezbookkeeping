<template>
    <v-row>
        <v-col cols="12">
            <v-card :class="{ disabled: loading || saving }">
                <template #title>
                    <span>Bank SMS Automation</span>
                    <v-progress-circular indeterminate size="20" class="ms-3" v-if="loading || saving"/>
                </template>

                <v-card-text>
                    <v-alert type="info" variant="tonal" class="mb-5">
                        The iPhone Shortcut sends only the bank message. Account identifiers below decide which
                        ezBookkeeping account receives the transaction. Categories always come from your current categories.
                    </v-alert>

                    <v-switch
                        color="primary"
                        label="Enable this user for bank SMS ingestion"
                        :disabled="loading || saving"
                        v-model="form.enabled"
                    />

                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12">
            <v-card title="Account identification rules" :class="{ disabled: loading || saving }">
                <v-card-text>
                    <v-alert type="warning" variant="tonal" class="mb-5">
                        Use text that appears in the SMS, such as a card suffix or masked account number. Identifiers
                        are case-insensitive and must contain at least four characters.
                    </v-alert>

                    <v-row v-for="(mapping, index) in form.accountMappings" :key="index" align="center">
                        <v-col cols="12" md="5">
                            <v-text-field
                                label="SMS identifier"
                                placeholder="Card ending 1234"
                                maxlength="64"
                                :disabled="loading || saving"
                                v-model="mapping.identifier"
                            />
                        </v-col>
                        <v-col cols="10" md="6">
                            <v-select
                                item-title="title"
                                item-value="id"
                                label="Account"
                                :items="accountOptions"
                                :disabled="loading || saving"
                                v-model="mapping.accountId"
                            />
                        </v-col>
                        <v-col cols="2" md="1" class="text-end">
                            <v-btn icon variant="text" color="error" :disabled="loading || saving" @click="removeMapping(index)">
                                <v-icon :icon="mdiDeleteOutline"/>
                            </v-btn>
                        </v-col>
                    </v-row>

                    <v-btn variant="tonal" :prepend-icon="mdiPlus" :disabled="loading || saving" @click="addMapping">
                        Add account rule
                    </v-btn>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12">
            <v-card title="Prompt rules" :class="{ disabled: loading || saving }">
                <v-card-text>
                    <v-textarea
                        rows="12"
                        auto-grow
                        counter="8000"
                        maxlength="8000"
                        label="Administrator instructions"
                        :disabled="loading || saving"
                        v-model="form.prompt"
                    />
                    <v-btn variant="text" :disabled="loading || saving" @click="resetPrompt">Reset to default</v-btn>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12">
            <v-card title="Test without creating a transaction" :class="{ disabled: loading || previewing }">
                <v-card-text>
                    <v-textarea
                        rows="4"
                        label="Bank message"
                        :disabled="loading || previewing"
                        v-model="testMessage"
                    />
                    <v-btn color="secondary" :disabled="loading || previewing || !testMessage.trim()" @click="preview">
                        Preview
                        <v-progress-circular indeterminate size="22" class="ms-2" v-if="previewing"/>
                    </v-btn>

                    <v-alert type="success" variant="tonal" class="mt-5" v-if="previewResult">
                        <pre class="preview-result">{{ JSON.stringify(previewResult, null, 2) }}</pre>
                    </v-alert>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12">
            <v-card title="Bank SMS outbox">
                <template #append>
                    <v-btn icon variant="text" :loading="loadingOutbox" @click="loadOutbox">
                        <v-icon :icon="mdiRefresh"/>
                    </v-btn>
                </template>
                <v-card-text>
                    <v-alert type="info" variant="tonal" class="mb-4">
                        Messages are saved here immediately, then processed in the background with up to three retries.
                    </v-alert>
                    <v-table density="comfortable">
                        <thead>
                            <tr>
                                <th>Status</th>
                                <th>Retries</th>
                                <th>Received</th>
                                <th>Message</th>
                                <th>Last error</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr v-for="item in outboxItems" :key="item.id">
                                <td><v-chip size="small" :color="outboxStatusColor(item.status)">{{ item.status }}</v-chip></td>
                                <td>{{ item.retryCount }}/3</td>
                                <td class="text-no-wrap">{{ formatOutboxTime(item.createdUnixTime) }}</td>
                                <td class="outbox-message">{{ item.message }}</td>
                                <td class="outbox-error">{{ item.lastError || '—' }}</td>
                            </tr>
                            <tr v-if="!loadingOutbox && outboxItems.length === 0">
                                <td colspan="5" class="text-center text-medium-emphasis">No messages received yet.</td>
                            </tr>
                        </tbody>
                    </v-table>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12">
            <v-btn color="primary" size="large" :disabled="loading || saving" @click="save">
                Save Bank SMS Settings
                <v-progress-circular indeterminate size="22" class="ms-2" v-if="saving"/>
            </v-btn>
        </v-col>
    </v-row>

    <snack-bar ref="snackbar"/>
</template>

<script setup lang="ts">
import SnackBar from '@/components/desktop/SnackBar.vue';

import { computed, onMounted, onUnmounted, reactive, ref, useTemplateRef } from 'vue';

import services from '@/lib/services.ts';

import type { AccountInfoResponse } from '@/models/account.ts';
import type {
    BankMessageAutomationSettingRequest,
    BankMessageAutomationSettingResponse,
    BankMessageOutboxItem,
    BankMessageOutboxStatus,
    BankMessageProcessResponse
} from '@/models/bank_message.ts';

import { mdiDeleteOutline, mdiPlus, mdiRefresh } from '@mdi/js';

type SnackBarType = InstanceType<typeof SnackBar>;

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const loading = ref(true);
const saving = ref(false);
const previewing = ref(false);
const loadingOutbox = ref(false);
const accounts = ref<AccountInfoResponse[]>([]);
const outboxItems = ref<BankMessageOutboxItem[]>([]);
const defaultPrompt = ref('');
const testMessage = ref('');
const previewResult = ref<BankMessageProcessResponse | null>(null);
let outboxRefreshTimer: number | undefined;

const form = reactive<BankMessageAutomationSettingRequest>({
    enabled: false,
    prompt: '',
    accountMappings: []
});

const accountOptions = computed(() => flattenAccounts(accounts.value).map(account => ({
    id: account.id,
    title: `${account.name} (${account.currency})`
})));

function flattenAccounts(items: AccountInfoResponse[]): AccountInfoResponse[] {
    const result: AccountInfoResponse[] = [];

    for (const account of items) {
        if (account.subAccounts?.length) {
            result.push(...flattenAccounts(account.subAccounts));
        } else if (!account.hidden) {
            result.push(account);
        }
    }

    return result;
}

function applySettings(settings: BankMessageAutomationSettingResponse): void {
    form.enabled = settings.enabled;
    form.prompt = settings.prompt;
    form.accountMappings = settings.accountMappings.map(mapping => ({ ...mapping }));
    defaultPrompt.value = settings.defaultPrompt;
}

function addMapping(): void {
    form.accountMappings.push({
        identifier: '',
        accountId: ''
    });
}

function removeMapping(index: number): void {
    form.accountMappings.splice(index, 1);
}

function resetPrompt(): void {
    form.prompt = defaultPrompt.value;
}

function load(): void {
    loading.value = true;

    Promise.all([
        services.getAllAccounts({ visibleOnly: true }),
        services.getBankMessageSettings()
    ]).then(([accountResponse, settingResponse]) => {
        accounts.value = accountResponse.data.result;
        applySettings(settingResponse.data.result);
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function loadOutbox(): void {
    if (loadingOutbox.value) {
        return;
    }

    loadingOutbox.value = true;
    services.getBankMessageOutbox().then(response => {
        outboxItems.value = response.data.result;
        loadingOutbox.value = false;
    }).catch(error => {
        loadingOutbox.value = false;
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function formatOutboxTime(unixTime: number): string {
    return new Date(unixTime * 1000).toLocaleString();
}

function outboxStatusColor(status: BankMessageOutboxStatus): string {
    if (status === 'succeeded') return 'success';
    if (status === 'failed') return 'error';
    if (status === 'retrying') return 'warning';
    if (status === 'processing') return 'info';
    if (status === 'ignored') return 'secondary';
    return 'default';
}

function save(): void {
    if (saving.value) {
        return;
    }

    const invalidMapping = form.accountMappings.some(mapping => mapping.identifier.trim().length < 4 || !mapping.accountId);

    if (invalidMapping) {
        snackbar.value?.showMessage('Every account rule needs an identifier of at least four characters and an account.');
        return;
    }

    saving.value = true;

    services.updateBankMessageSettings({
        enabled: form.enabled,
        prompt: form.prompt,
        accountMappings: form.accountMappings.map(mapping => ({
            identifier: mapping.identifier.trim(),
            accountId: mapping.accountId
        }))
    }).then(response => {
        applySettings(response.data.result);
        saving.value = false;
        snackbar.value?.showMessage('Bank SMS settings saved.');
    }).catch(error => {
        saving.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function preview(): void {
    if (previewing.value || !testMessage.value.trim()) {
        return;
    }

    previewing.value = true;
    previewResult.value = null;

    services.previewBankMessage(testMessage.value.trim()).then(response => {
        previewResult.value = response.data.result;
        previewing.value = false;
    }).catch(error => {
        previewing.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

onMounted(() => {
    load();
    loadOutbox();
    outboxRefreshTimer = window.setInterval(loadOutbox, 5000);
});

onUnmounted(() => {
    if (outboxRefreshTimer !== undefined) {
        window.clearInterval(outboxRefreshTimer);
    }
});
</script>

<style scoped>
.preview-result {
    margin: 0;
    overflow-x: auto;
    white-space: pre-wrap;
}

.outbox-message,
.outbox-error {
    max-width: 320px;
    white-space: pre-wrap;
    word-break: break-word;
}
</style>

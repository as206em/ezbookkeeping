<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ disabled: loading || saving }" :back-link="tt('Back')"/>
            <f7-nav-title title="Bank SMS Automation"/>
        </f7-navbar>

        <f7-block class="margin-vertical-half">
            Your Shortcut sends only the bank message. Account rules decide where the transaction is saved;
            categories always come from your current ezBookkeeping categories.
        </f7-block>

        <f7-list strong inset dividers :class="{ disabled: loading || saving }">
            <f7-list-item title="Enable bank SMS ingestion">
                <template #after>
                    <f7-toggle :checked="form.enabled" @toggle:change="form.enabled = $event"/>
                </template>
            </f7-list-item>
        </f7-list>

        <f7-block-title>Account identification rules</f7-block-title>
        <f7-block class="margin-vertical-half">
            Add text found in the SMS, such as a card suffix. Identifiers are case-insensitive and need at least four characters.
        </f7-block>
        <f7-list strong inset dividers :class="{ disabled: loading || saving }">
            <template :key="index" v-for="(mapping, index) in form.accountMappings">
                <f7-list-input type="text" label="SMS identifier" placeholder="Card ending 1234"
                               maxlength="64" v-model:value="mapping.identifier"/>
                <f7-list-input type="select" label="Account" :value="mapping.accountId"
                               @input="mapping.accountId = $event.target.value">
                    <option value="">Choose an account</option>
                    <option :value="account.id" :key="account.id" v-for="account in accountOptions">
                        {{ account.title }}
                    </option>
                </f7-list-input>
                <f7-list-button color="red" @click="removeMapping(index)">Remove rule</f7-list-button>
            </template>
            <f7-list-button @click="addMapping">Add account rule</f7-list-button>
        </f7-list>

        <f7-block-title>Prompt rules</f7-block-title>
        <f7-list strong inset :class="{ disabled: loading || saving }">
            <f7-list-input type="textarea" label="Administrator instructions" :value="form.prompt"
                           maxlength="8000" resizable @input="form.prompt = $event.target.value"/>
            <f7-list-button @click="resetPrompt">Reset to default</f7-list-button>
        </f7-list>

        <f7-block-title>Test without creating a transaction</f7-block-title>
        <f7-list strong inset :class="{ disabled: loading || previewing }">
            <f7-list-input type="textarea" label="Bank message" :value="testMessage" resizable
                           @input="testMessage = $event.target.value"/>
            <f7-list-button :class="{ disabled: !testMessage.trim() }" @click="preview">Preview</f7-list-button>
        </f7-list>
        <f7-block strong inset v-if="previewResult">
            <pre class="bank-message-preview">{{ JSON.stringify(previewResult, null, 2) }}</pre>
        </f7-block>

        <f7-block-title>Bank SMS outbox</f7-block-title>
        <f7-block class="margin-vertical-half">
            Messages are saved immediately and processed in the background with up to three retries.
        </f7-block>
        <f7-list strong inset media-list>
            <f7-list-item v-for="item in outboxItems" :key="item.id"
                          :title="item.status" :subtitle="`${item.retryCount}/3 retries · ${formatOutboxTime(item.createdUnixTime)}`">
                <template #text>
                    <div class="bank-message-outbox-text">{{ item.message }}</div>
                    <div class="text-color-red" v-if="item.lastError">{{ item.lastError }}</div>
                </template>
            </f7-list-item>
            <f7-list-item title="No messages received yet" v-if="!loadingOutbox && outboxItems.length === 0"/>
            <f7-list-button @click="loadOutbox">{{ loadingOutbox ? 'Refreshing…' : 'Refresh outbox' }}</f7-list-button>
        </f7-list>

        <f7-list strong inset>
            <f7-list-button fill :class="{ disabled: loading || saving }" @click="save">Save Bank SMS Settings</f7-list-button>
        </f7-list>
    </f7-page>
</template>

<script setup lang="ts">
import { computed, onUnmounted, reactive, ref } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { hideLoading, showLoading, useI18nUIComponents } from '@/lib/ui/mobile.ts';
import services from '@/lib/services.ts';

import type { AccountInfoResponse } from '@/models/account.ts';
import type {
    BankMessageAutomationSettingRequest,
    BankMessageAutomationSettingResponse,
    BankMessageOutboxItem,
    BankMessageProcessResponse
} from '@/models/bank_message.ts';

const props = defineProps<{ f7router: Router.Router }>();

const { tt } = useI18n();
const { routeBackOnError, showToast } = useI18nUIComponents();

const loading = ref(true);
const saving = ref(false);
const previewing = ref(false);
const loadingOutbox = ref(false);
const loadingError = ref<unknown | null>(null);
const accounts = ref<AccountInfoResponse[]>([]);
const defaultPrompt = ref('');
const testMessage = ref('');
const previewResult = ref<BankMessageProcessResponse | null>(null);
const outboxItems = ref<BankMessageOutboxItem[]>([]);
const outboxRefreshTimer = window.setInterval(loadOutbox, 5000);

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
    form.accountMappings.push({ identifier: '', accountId: '' });
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
        loadingError.value = error;

        if (!error.processed) {
            showToast(error.message || error);
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
            showToast(error.message || error);
        }
    });
}

function formatOutboxTime(unixTime: number): string {
    return new Date(unixTime * 1000).toLocaleString();
}

function save(): void {
    const invalidMapping = form.accountMappings.some(mapping => mapping.identifier.trim().length < 4 || !mapping.accountId);

    if (invalidMapping) {
        showToast('Every account rule needs an identifier of at least four characters and an account.');
        return;
    }

    saving.value = true;
    showLoading(() => saving.value);

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
        hideLoading();
        showToast('Bank SMS settings saved.');
    }).catch(error => {
        saving.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function preview(): void {
    if (!testMessage.value.trim()) {
        return;
    }

    previewing.value = true;
    previewResult.value = null;
    showLoading(() => previewing.value);

    services.previewBankMessage(testMessage.value.trim()).then(response => {
        previewResult.value = response.data.result;
        previewing.value = false;
        hideLoading();
    }).catch(error => {
        previewing.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

load();
loadOutbox();

onUnmounted(() => window.clearInterval(outboxRefreshTimer));
</script>

<style scoped>
.bank-message-preview {
    margin: 0;
    overflow-x: auto;
    white-space: pre-wrap;
}

.bank-message-outbox-text {
    white-space: pre-wrap;
    word-break: break-word;
}
</style>

<template>
    <v-dialog persistent width="560" v-model="showState">
        <v-card class="dialog-layout-card">
            <v-toolbar color="primary">
                <v-toolbar-title>Connect Binance</v-toolbar-title>
            </v-toolbar>

            <v-card-text class="pa-4">
                <ol class="text-body-medium text-medium-emphasis mb-4 ps-4">
                    <li>In Binance, open API Management and create an API key.</li>
                    <li>Turn on <strong>Enable Reading</strong> only. Leave trading and withdrawals off, keys that can trade or withdraw are refused.</li>
                    <li>If you can, restrict the key to the IP address of this server.</li>
                    <li>Paste the key and secret below. They are stored encrypted and never shown again.</li>
                </ol>

                <v-text-field class="mb-3" persistent-placeholder autocomplete="off" label="API key" :disabled="saving" v-model="apiKey" />
                <v-text-field class="mb-1" persistent-placeholder autocomplete="off" type="password" label="API secret" :disabled="saving"
                              v-model="apiSecret" @keyup.enter="connect" />

                <v-alert type="error" variant="tonal" density="compact" class="mt-2" v-if="errorMessage">{{ errorMessage }}</v-alert>
            </v-card-text>

            <v-card-actions class="px-4 pb-4">
                <v-spacer />
                <v-btn color="gray" :disabled="saving" @click="cancel">Cancel</v-btn>
                <v-btn color="primary" :disabled="saving" @click="connect">{{ saving ? 'Checking key…' : 'Connect' }}</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useAssetsStore } from '@/stores/asset.ts';

const assetsStore = useAssetsStore();

let resolveFunc: (() => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const saving = ref<boolean>(false);
const apiKey = ref<string>('');
const apiSecret = ref<string>('');
const errorMessage = ref<string>('');

function open(): Promise<void> {
    apiKey.value = '';
    apiSecret.value = '';
    errorMessage.value = '';
    saving.value = false;
    showState.value = true;

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function connect(): void {
    if (!apiKey.value.trim() || !apiSecret.value.trim()) {
        errorMessage.value = 'Enter both the API key and the secret';
        return;
    }

    errorMessage.value = '';
    saving.value = true;

    assetsStore.connectBinance({ apiKey: apiKey.value.trim(), apiSecret: apiSecret.value.trim() }).then(() => {
        saving.value = false;
        apiKey.value = '';
        apiSecret.value = '';
        showState.value = false;
        resolveFunc?.();
    }).catch((error: { message?: string; error?: { errorMessage?: string } }) => {
        saving.value = false;
        errorMessage.value = error.error?.errorMessage || error.message || 'Unable to connect Binance';
    });
}

function cancel(): void {
    apiKey.value = '';
    apiSecret.value = '';
    showState.value = false;
    rejectFunc?.();
}

defineExpose({
    open
});
</script>

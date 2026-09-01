import type { TransactionInfoResponse } from '@/models/transaction.ts';

export interface BankMessageAccountMapping {
    identifier: string;
    accountId: string;
}

export interface BankMessageAutomationSettingResponse {
    enabled: boolean;
    prompt: string;
    defaultPrompt: string;
    accountMappings: BankMessageAccountMapping[];
}

export interface BankMessageAutomationSettingRequest {
    enabled: boolean;
    prompt: string;
    accountMappings: BankMessageAccountMapping[];
}

export interface RecognizedBankMessage {
    amount: string;
    currency: string;
    isDeclined: boolean;
    transactionType: 'income' | 'expense';
    category: string;
    transactionTime: string;
    remark: string;
    storeName: string;
}

export interface BankMessageAIPreview {
    systemPrompt: string;
    userPrompt: string;
    rawResponse: string;
}

export interface BankMessageProcessResponse {
    created: boolean;
    reason?: string;
    previewError?: string;
    matchedAccountId?: string;
    recognized?: RecognizedBankMessage;
    transaction?: TransactionInfoResponse;
    aiPreview?: BankMessageAIPreview;
}

export type BankMessageOutboxStatus = 'queued' | 'processing' | 'retrying' | 'paused' | 'succeeded' | 'ignored' | 'failed';

export interface BankMessageOutboxItem {
    id: string;
    status: BankMessageOutboxStatus;
    retryCount: number;
    message: string;
    lastError?: string;
    transactionId?: string;
    createdUnixTime: number;
    updatedUnixTime: number;
}

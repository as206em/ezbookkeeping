import { describe, expect, test } from 'vitest';

import { CategoryType } from '@/core/category.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';

import { getMerchantFromTransactionComment, getMerchantKey, getMerchantToRemember } from '@/lib/category_merchant.ts';

function category(id: string, parentId: string, type: CategoryType = CategoryType.Expense): TransactionCategory {
    return { id, parentId, type, name: id } as TransactionCategory;
}

const noRules = () => undefined;

describe('getMerchantFromTransactionComment', () => {
    test('takes the merchant part of a bank SMS comment', () => {
        expect(getMerchantFromTransactionComment('Carrefour - Groceries')).toBe('Carrefour');
        expect(getMerchantFromTransactionComment('  Talabat   Mart  ')).toBe('Talabat Mart');
        expect(getMerchantFromTransactionComment('')).toBe('');
        expect(getMerchantFromTransactionComment('x'.repeat(65))).toBe('');
    });

    test('builds case-insensitive keys', () => {
        expect(getMerchantKey(' CARREFOUR  Market ')).toBe('carrefour market');
    });
});

describe('getMerchantToRemember', () => {
    test('offers the merchant when a secondary category changes', () => {
        expect(getMerchantToRemember('Carrefour - Food', 'restaurants', category('groceries', 'food'), noRules)).toBe('Carrefour');
    });

    test('skips unchanged, primary, transfer and already remembered categories', () => {
        expect(getMerchantToRemember('Carrefour', 'groceries', category('groceries', 'food'), noRules)).toBe('');
        expect(getMerchantToRemember('Carrefour', 'restaurants', category('food', '0'), noRules)).toBe('');
        expect(getMerchantToRemember('Carrefour', 'restaurants', category('out', 'transfers', CategoryType.Transfer), noRules)).toBe('');
        expect(getMerchantToRemember('Carrefour', 'restaurants', category('groceries', 'food'), () => 'groceries')).toBe('');
        expect(getMerchantToRemember('', 'restaurants', category('groceries', 'food'), noRules)).toBe('');
    });
});

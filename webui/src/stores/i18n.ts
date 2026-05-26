//
// File:        webui/src/stores/i18n.ts
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

import { ref } from 'vue'
import { defineStore } from 'pinia'

type Locale = 'en' | 'de'

const translations: Record<Locale, Record<string, string>> = {
  en: {
    // TopBar
    title: 'KOsync',
    logout: 'Log Out',
    login: 'Login',
    logged_in_as: 'Logged in as {0}',

    // HomeView / Setup
    setup_title: 'KOsync Setup',
    setup_desc: 'To synchronize your KOReader reading progress, use the official KOReader Sync plugin with this server.',
    server_address: 'Server Address',
    auth_method: 'Authentication Method',
    auth_method_desc: 'Enter the following credentials in KOReader:',
    username: 'Username',
    password: 'Password',
    api_token_title: 'Alternatively, configure via Custom API Token',
    api_token_desc: 'In KOReader setup, you can also use this token instead of Basic Auth:',
    api_token: 'API Token',
    my_documents: 'My documents',
    login_prompt: 'Please login to see your documents.',
    setup_koreader_title: 'How to Setup KOReader Sync',
    setup_step_1: 'Set <strong>Custom sync server</strong> to <code>{0}</code>',
    setup_step_2: '<strong>Register</strong> a new account.',
    setup_step_3: 'Enable <strong>automatically keep documents in sync</strong>.',
    setup_step_4: 'Set <strong>periodically sync every # pages</strong> to 2.',
    setup_step_5: 'Set <strong>Document matching method</strong> to "Binary".',
    setup_step_6: 'Login on all devices repeating steps 3-5.',
    setup_step_7: 'Login to KOSync',
    setup_step_8: 'Read books.',

    // DashboardMetrics
    total_books: 'Total Books',
    finished_books: 'Finished Books',
    reading_progress: 'Reading Progress',
    reading_time: 'Total Reading Time',
    total_documents: 'Total Documents',
    average_progress: 'Average Progress',
    recent_read_time: 'Recent Read Time',
    minutes: 'Minutes',
    minutes_abbr: 'min',

    // ReadStatisticsChart
    reading_activity: 'Reading Activity',
    minutes_read: 'Minutes Read',
    no_activity: 'No reading activity recorded yet.',
    chart_updates: 'Updates',
    chart_progress_increase: 'Progress Increase (%)',
    chart_reading_time: 'Reading Time (min)',
    chart_number_of_updates: 'Number of Updates',
    chart_title: 'Reading Statistics (Last {0} Days)',
    chart_loading: 'Loading statistics...',

    // DocumentsList
    search_placeholder: 'Search documents...',
    col_title: 'Title',
    col_progress: 'Progress',
    col_last_read: 'Last Read',
    col_actions: 'Actions',
    btn_show_history: 'Show History',
    btn_delete_doc: 'Delete Document',
    delete_doc_confirm: 'Are you sure you want to delete "{0}"?',
    confirm_title: 'Confirm',
    cancel: 'Cancel',
    delete: 'Delete',
    no_documents: 'No documents found.',
    view_grid: 'Grid',
    view_list: 'List',

    // HistoryList
    history_title: 'Reading History',
    col_percentage: 'Percentage',
    col_page: 'Page',
    col_timestamp: 'Timestamp',
    col_device: 'Device',
    btn_restore: 'Restore',
    btn_delete_history: 'Delete history entry',
    delete_history_confirm: 'Are you sure you want to delete this history item from "{0}"?',
    restore_history_confirm: 'Are you sure you want to restore the document to the state from "{0}"?',
    col_previous_title: 'Previous Title',
    col_when: 'When',
    no_history: 'No history found.',
    no_history_desc: 'This document does not have a history.<br>You can try pushing your progress and you might want to check your automatic push setting.',

    // LoginModal
    login_header: 'Login',
    err_fields_required: 'Please enter both username and password.',
    err_invalid_credentials: 'Invalid username or password.',
    err_login_failed: 'An error occurred during login.',
  },
  de: {
    // TopBar
    title: 'KOsync',
    logout: 'Abmelden',
    login: 'Anmelden',
    logged_in_as: 'Angemeldet als {0}',

    // HomeView / Setup
    setup_title: 'KOsync Einrichtung',
    setup_desc: 'Um deinen Lesefortschritt in KOReader zu synchronisieren, verwende das offizielle KOReader Sync-Plugin mit diesem Server.',
    server_address: 'Serveradresse',
    auth_method: 'Authentifizierungsmethode',
    auth_method_desc: 'Gib die folgenden Zugangsdaten in KOReader ein:',
    username: 'Benutzername',
    password: 'Passwort',
    api_token_title: 'Alternativ über benutzerdefinierten API-Token konfigurieren',
    api_token_desc: 'Bei der KOReader-Einrichtung kannst du auch diesen Token anstelle von Basic Auth verwenden:',
    api_token: 'API-Token',
    my_documents: 'Meine Dokumente',
    login_prompt: 'Bitte melde dich an, um deine Dokumente zu sehen.',
    setup_koreader_title: 'KOReader Sync einrichten',
    setup_step_1: 'Setze den <strong>Benutzerdefinierten Sync-Server</strong> auf <code>{0}</code>',
    setup_step_2: '<strong>Registriere</strong> ein neues Konto.',
    setup_step_3: 'Aktiviere <strong>Dokumente automatisch synchron halten</strong>.',
    setup_step_4: 'Setze <strong>periodisch synchronisieren alle # Seiten</strong> auf 2.',
    setup_step_5: 'Setze <strong>Dokumenten-Abgleichsmethode</strong> auf "Binär".',
    setup_step_6: 'Melde dich auf allen Geräten an und wiederhole die Schritte 3-5.',
    setup_step_7: 'Melde dich bei KOsync an',
    setup_step_8: 'Bücher lesen.',

    // DashboardMetrics
    total_books: 'Bücher insgesamt',
    finished_books: 'Gelesene Bücher',
    reading_progress: 'Lesefortschritt',
    reading_time: 'Gesamte Lesezeit',
    total_documents: 'Dokumente insgesamt',
    average_progress: 'Durchschnittlicher Fortschritt',
    recent_read_time: 'Kürzliche Lesezeit',
    minutes: 'Minuten',
    minutes_abbr: 'Min.',

    // ReadStatisticsChart
    reading_activity: 'Leseaktivität',
    minutes_read: 'Gelesene Minuten',
    no_activity: 'Noch keine Leseaktivität aufgezeichnet.',
    chart_updates: 'Updates',
    chart_progress_increase: 'Fortschrittserhöhung (%)',
    chart_reading_time: 'Lesezeit (Min.)',
    chart_number_of_updates: 'Anzahl der Updates',
    chart_title: 'Lesestatistiken (Letzte {0} Tage)',
    chart_loading: 'Statistiken werden geladen...',

    // DocumentsList
    search_placeholder: 'Dokumente suchen...',
    col_title: 'Titel',
    col_progress: 'Fortschritt',
    col_last_read: 'Zuletzt gelesen',
    col_actions: 'Aktionen',
    btn_show_history: 'Verlauf anzeigen',
    btn_delete_doc: 'Dokument löschen',
    delete_doc_confirm: 'Bist du sicher, dass du "{0}" löschen möchtest?',
    confirm_title: 'Bestätigen',
    cancel: 'Abbrechen',
    delete: 'Löschen',
    no_documents: 'Keine Dokumente gefunden.',
    view_grid: 'Raster',
    view_list: 'Liste',

    // HistoryList
    history_title: 'Leseverlauf',
    col_percentage: 'Prozentsatz',
    col_page: 'Seite',
    col_timestamp: 'Zeitstempel',
    col_device: 'Gerät',
    btn_restore: 'Wiederherstellen',
    btn_delete_history: 'Verlaufseintrag löschen',
    delete_history_confirm: 'Bist du sicher, dass du diesen Verlaufseintrag vom {0} löschen möchtest?',
    restore_history_confirm: 'Bist du sicher, dass du das Dokument auf den Stand vom {0} zurücksetzen möchtest?',
    col_previous_title: 'Vorheriger Titel',
    col_when: 'Wann',
    no_history: 'Kein Verlauf gefunden.',
    no_history_desc: 'Dieses Dokument hat keinen Verlauf.<br>Du kannst versuchen, deinen Fortschritt hochzuladen und deine automatische Upload-Einstellung zu überprüfen.',

    // LoginModal
    login_header: 'Anmelden',
    err_fields_required: 'Bitte gib sowohl den Benutzernamen als auch das Passwort ein.',
    err_invalid_credentials: 'Ungültiger Benutzername oder Passwort.',
    err_login_failed: 'Bei der Anmeldung ist ein Fehler aufgetreten.',
  }
}

export const useI18nStore = defineStore('i18n', () => {
  const getBrowserLanguage = (): Locale => {
    if (typeof navigator !== 'undefined') {
      const lang = navigator.language || (navigator as any).userLanguage || 'en'
      return lang.toLowerCase().startsWith('de') ? 'de' : 'en'
    }
    return 'en'
  }

  const savedLocale = typeof localStorage !== 'undefined' ? localStorage.getItem('locale') as Locale | null : null
  const locale = ref<Locale>(savedLocale || getBrowserLanguage())

  function setLocale(newLocale: Locale) {
    locale.value = newLocale
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem('locale', newLocale)
    }
  }

  function t(key: string, ...args: any[]): string {
    const activeDict = translations[locale.value] || translations['en']
    let text = activeDict[key] || translations['en'][key] || key
    if (args.length > 0) {
      args.forEach((arg, index) => {
        text = text.replace(new RegExp(`\\{${index}\\}`, 'g'), String(arg))
        text = text.replace('%s', String(arg))
      })
    }
    return text
  }

  return { locale, setLocale, t }
})

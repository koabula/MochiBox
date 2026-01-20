<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { Activity, Server, Wifi, Globe, Clock, Copy, Plus, Link as LinkIcon, Zap } from 'lucide-vue-next';
import api from '@/api';
import { useToastStore } from '@/stores/toast';
import { useSettingsStore } from '@/stores/settings';
import { useNetworkStore } from '@/stores/network';

const toastStore = useToastStore();
const settingsStore = useSettingsStore();
const networkStore = useNetworkStore();
const peerList = ref<any[]>([]);
const loading = ref(true);
const showConnectModal = ref(false);
const connectInput = ref('');
const connecting = ref(false);
let intervalId: any;

const fetchPeers = async () => {
    try {
        const peersRes = await api.get('/system/peers');
        peerList.value = peersRes.data;
    } catch (e) {
        console.error("Failed to fetch peers", e);
    } finally {
        loading.value = false;
    }
};

const handleConnect = async () => {
    if (!connectInput.value) return;
    connecting.value = true;
    try {
        await api.post('/system/connect', { multiaddr: connectInput.value });
        toastStore.success('Connected to peer successfully');
        showConnectModal.value = false;
        connectInput.value = '';
        fetchPeers(); // Refresh list
    } catch (e: any) {
        toastStore.error(e.response?.data?.error || 'Failed to connect');
    } finally {
        connecting.value = false;
    }
};

const handleBoost = async () => {
    const toastId = toastStore.info('Boost Network Started');
    try {
        await api.post('/system/bootstrap');
        toastStore.success('Boost Network Finished');
    } catch (e: any) {
        if (e.response && e.response.status === 409) {
             toastStore.remove(toastId);
             toastStore.warning('Boost Network is running');
        } else {
            console.error(e);
            toastStore.error('Boost failed');
        }
    }
};

onMounted(() => {
    fetchPeers();
    intervalId = setInterval(fetchPeers, 5000);
});

onUnmounted(() => {
    if (intervalId) clearInterval(intervalId);
});

const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toastStore.success("Copied to clipboard");
};
</script>

<template>
    <div class="p-8 max-w-6xl mx-auto space-y-8 animate-fade-in">
        
        <!-- Header -->
        <div class="flex items-center gap-4">
            <div class="p-3 bg-nord-10/10 rounded-xl">
                <Activity class="w-8 h-8 text-nord-10" />
            </div>
            <div>
                <h1 class="text-2xl font-bold text-nord-0 dark:text-nord-6">Network Status</h1>
                <p class="text-nord-3 dark:text-nord-4">Manage your IPFS node connectivity and peers</p>
            </div>
        </div>
        
        <div class="flex justify-end gap-3">
            <button @click="handleBoost" class="flex items-center gap-2 bg-white dark:bg-nord-1 hover:bg-nord-6 dark:hover:bg-nord-2 text-nord-1 dark:text-nord-6 border border-nord-4 dark:border-nord-2 px-4 py-2 rounded-lg font-medium transition-colors shadow-sm" title="This will try to connect more peers">
                <Zap class="w-4 h-4 text-amber-500" /> Boost Network
            </button>
            <button @click="showConnectModal = true" class="flex items-center gap-2 bg-nord-10 hover:bg-nord-9 text-white px-4 py-2 rounded-lg font-medium transition-colors shadow-sm">
                <Plus class="w-4 h-4" /> Connect Peer
            </button>
        </div>

        <!-- Node Status Cards -->
        <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
            <!-- Status Card -->
            <div class="bg-white dark:bg-nord-1 p-6 rounded-2xl border border-nord-4 dark:border-nord-2 shadow-sm flex flex-col justify-between">
                <div class="flex justify-between items-start">
                    <div>
                        <p class="text-sm font-bold text-nord-3 dark:text-nord-4 uppercase tracking-wider">Node Status</p>
                        <h3 class="text-3xl font-bold mt-2" :class="networkStore.isStarting ? 'text-amber-500' : 'text-nord-0 dark:text-nord-6'">
                            {{ networkStore.status.online ? 'Online' : (networkStore.isStarting ? 'Initializing...' : 'Offline') }}
                        </h3>
                    </div>
                    <div :class="networkStore.status.online ? 'bg-green-100 text-green-600' : (networkStore.isStarting ? 'bg-amber-100 text-amber-600' : 'bg-red-100 text-red-600')" class="p-2 rounded-lg">
                        <Wifi class="w-6 h-6" />
                    </div>
                </div>
                <div class="mt-4 flex items-center gap-2 text-sm text-nord-3">
                    <span class="w-2 h-2 rounded-full" :class="networkStore.status.online ? 'bg-green-500 animate-pulse' : (networkStore.isStarting ? 'bg-amber-500 animate-ping' : 'bg-red-500')"></span>
                    <span v-if="networkStore.isStarting">Starting IPFS Daemon...</span>
                    <span v-else-if="settingsStore.useEmbeddedNode && networkStore.status.online">Built-in Node Running</span>
                    <span v-else>{{ networkStore.status.online ? 'Connected to External Node' : 'Disconnected' }}</span>
                </div>
            </div>

            <!-- Peers Card -->
            <div class="bg-white dark:bg-nord-1 p-6 rounded-2xl border border-nord-4 dark:border-nord-2 shadow-sm flex flex-col justify-between">
                <div class="flex justify-between items-start">
                    <div>
                        <p class="text-sm font-bold text-nord-3 dark:text-nord-4 uppercase tracking-wider">Connected Peers</p>
                        <h3 class="text-3xl font-bold mt-2 text-nord-0 dark:text-nord-6">
                            {{ networkStore.status.peers }}
                        </h3>
                    </div>
                    <div class="bg-nord-10/10 text-nord-10 p-2 rounded-lg">
                        <Globe class="w-6 h-6" />
                    </div>
                </div>
                <div class="mt-4 text-sm text-nord-3">
                    Active connections
                </div>
            </div>

             <!-- Uptime/Info Card -->
             <div class="bg-white dark:bg-nord-1 p-6 rounded-2xl border border-nord-4 dark:border-nord-2 shadow-sm flex flex-col justify-between">
                <div class="flex justify-between items-start">
                    <div>
                        <p class="text-sm font-bold text-nord-3 dark:text-nord-4 uppercase tracking-wider">Protocol</p>
                        <h3 class="text-xl font-bold mt-2 text-nord-0 dark:text-nord-6">
                           IPFS / Libp2p
                        </h3>
                    </div>
                    <div class="bg-orange-100 text-orange-600 p-2 rounded-lg">
                        <Server class="w-6 h-6" />
                    </div>
                </div>
                 <div class="mt-4 text-sm text-nord-3 truncate" :title="networkStore.status.peer_id">
                    ID: {{ networkStore.status.peer_id ? networkStore.status.peer_id.slice(0, 15) + '...' : 'Unknown' }}
                </div>
            </div>
        </div>

        <!-- Details Section -->
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
            
            <!-- Peer List -->
            <div class="bg-white dark:bg-nord-1 rounded-2xl border border-nord-4 dark:border-nord-2 shadow-sm overflow-hidden flex flex-col h-[500px]">
                <div class="p-6 border-b border-nord-4 dark:border-nord-2 flex justify-between items-center">
                    <h3 class="font-bold text-lg text-nord-0 dark:text-nord-6">Peer List</h3>
                    <span class="text-xs font-mono bg-nord-6 dark:bg-nord-0 px-2 py-1 rounded text-nord-3">{{ peerList.length }} peers</span>
                </div>
                <div class="overflow-y-auto flex-1 p-0">
                    <div v-if="networkStore.isStarting" class="h-full flex flex-col items-center justify-center text-nord-3 gap-3">
                         <div class="w-8 h-8 border-2 border-nord-4 border-t-nord-10 rounded-full animate-spin"></div>
                         <p>Waiting for node to start...</p>
                    </div>
                    <table v-else class="w-full text-left border-collapse">
                        <thead class="bg-nord-6 dark:bg-nord-0 sticky top-0 z-10">
                            <tr>
                                <th class="p-4 text-xs font-bold text-nord-3 dark:text-nord-4 uppercase w-1/3">Peer ID</th>
                                <th class="p-4 text-xs font-bold text-nord-3 dark:text-nord-4 uppercase">Address</th>
                            </tr>
                        </thead>
                        <tbody class="divide-y divide-nord-4 dark:divide-nord-2">
                            <tr v-for="peer in peerList" :key="peer.id" class="hover:bg-nord-6 dark:hover:bg-nord-2 transition-colors">
                                <td class="p-4">
                                    <div class="flex items-center gap-2">
                                        <div class="w-8 h-8 rounded-full bg-gradient-to-br from-nord-7 to-nord-10 flex items-center justify-center text-white text-xs font-bold">
                                            {{ peer.id.substring(0,2) }}
                                        </div>
                                        <div class="flex-1 min-w-0">
                                            <div class="font-mono text-xs text-nord-1 dark:text-nord-5 truncate w-32" :title="peer.id">
                                                {{ peer.id }}
                                            </div>
                                        </div>
                                        <button @click="copyToClipboard(peer.id)" class="text-nord-3 hover:text-nord-10"><Copy class="w-3 h-3" /></button>
                                    </div>
                                </td>
                                <td class="p-4">
                                    <div class="font-mono text-xs text-nord-3 dark:text-nord-4 truncate max-w-[200px]" :title="peer.address">
                                        {{ peer.address || 'Relay / Unknown' }}
                                    </div>
                                </td>
                            </tr>
                            <tr v-if="peerList.length === 0">
                                <td colspan="2" class="p-8 text-center text-nord-3 dark:text-nord-4">
                                    No peers connected yet.
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </div>

            <!-- Addresses & Config -->
            <div class="space-y-6">
                <div class="bg-white dark:bg-nord-1 rounded-2xl border border-nord-4 dark:border-nord-2 shadow-sm p-6">
                    <h3 class="font-bold text-lg text-nord-0 dark:text-nord-6 mb-4">My Swarm Addresses</h3>
                    <div class="space-y-2 max-h-60 overflow-y-auto pr-2">
                        <div v-for="addr in networkStore.status.addresses" :key="addr" class="group flex items-center gap-2 p-2 bg-nord-6 dark:bg-nord-0 rounded-lg border border-nord-4 dark:border-nord-2 hover:border-nord-10 transition-colors">
                            <code class="flex-1 font-mono text-xs text-nord-2 dark:text-nord-4 break-all">
                                {{ addr.includes('/p2p/') ? addr : `${addr}/p2p/${networkStore.status.peer_id}` }}
                            </code>
                            <button @click="copyToClipboard(addr.includes('/p2p/') ? addr : `${addr}/p2p/${networkStore.status.peer_id}`)" class="opacity-0 group-hover:opacity-100 p-1 text-nord-3 hover:text-nord-10 transition-opacity">
                                <Copy class="w-4 h-4" />
                            </button>
                        </div>
                         <div v-if="!networkStore.status.addresses || networkStore.status.addresses.length === 0" class="text-nord-3 text-sm italic">
                            {{ networkStore.isStarting ? 'Waiting for node addresses...' : 'Generating addresses...' }}
                        </div>
                    </div>
                </div>

                <div class="bg-nord-10/5 rounded-2xl border border-nord-10/20 p-6">
                     <h3 class="font-bold text-lg text-nord-10 mb-2">Discovery Mode</h3>
                     <p class="text-sm text-nord-3 dark:text-nord-4 mb-4">
                        Your node is running in <strong>Server Mode</strong> with <strong>MDNS</strong> enabled. 
                        It will automatically discover other IPFS nodes on your local network and connect to the global DHT.
                     </p>
                     <div class="flex items-center gap-2 text-xs font-medium text-nord-10 bg-nord-10/10 w-fit px-3 py-1 rounded-full">
                        <div class="w-2 h-2 bg-nord-10 rounded-full animate-pulse"></div>
                        Listening for peers...
                     </div>
                </div>
            </div>

        </div>

    </div>

    <!-- Connect Modal -->
    <div v-if="showConnectModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" @click="showConnectModal = false">
        <div class="bg-white dark:bg-nord-3 rounded-lg shadow-xl w-full max-w-lg p-6 relative border border-nord-4 dark:border-nord-2" @click.stop>
            <h3 class="text-xl font-bold text-nord-0 dark:text-nord-6 mb-4 flex items-center gap-2">
                <LinkIcon class="w-5 h-5" />
                Connect to Peer
            </h3>
            
            <div class="space-y-4">
                <p class="text-sm text-nord-3 dark:text-nord-4">
                    Enter the full multiaddress of the peer you want to connect to.
                    <br>
                    <span class="text-xs font-mono bg-nord-6 dark:bg-nord-0 px-1 rounded">/ip4/1.2.3.4/tcp/4001/p2p/Qm...</span>
                </p>
                
                <input 
                    v-model="connectInput"
                    type="text" 
                    placeholder="/ip4/..."
                    class="w-full px-4 py-3 rounded-lg border border-nord-4 dark:border-nord-3 bg-white dark:bg-nord-0 text-nord-1 dark:text-nord-6 focus:ring-2 focus:ring-nord-10 outline-none transition-all font-mono text-sm"
                    @keyup.enter="handleConnect"
                />
            </div>

            <div class="mt-6 flex justify-end gap-3">
                <button @click="showConnectModal = false" class="px-4 py-2 text-nord-3 dark:text-nord-4 hover:bg-nord-6 dark:hover:bg-nord-2 rounded-lg transition-colors text-sm font-medium">
                    Cancel
                </button>
                <button 
                    @click="handleConnect" 
                    :disabled="!connectInput || connecting"
                    class="px-4 py-2 bg-nord-10 hover:bg-nord-9 disabled:opacity-50 text-white rounded-lg transition-colors text-sm font-medium flex items-center gap-2"
                >
                    <div v-if="connecting" class="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
                    Connect
                </button>
            </div>
        </div>
    </div>
</template>

import axios from 'axios';

const api = axios.create({
  baseURL: 'http://localhost:3666/api',
});

export const listFiles = async () => {
  const res = await api.get('/files');
  return res.data;
};

export const getNodeStatus = async () => {
  const res = await api.get('/system/status');
  return res.data;
};

export default api;

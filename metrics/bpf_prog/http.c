#include <linux/bpf.h>
#include "include/bpf_helpers.h"
#include "include/bpf_endian.h"
#include "include/types.h"
#include <linux/if_ether.h>
#include <linux/if_packet.h>
#include <linux/ip.h>
#include <linux/in.h>
#include <linux/string.h>
#include <linux/tcp.h>
#include <linux/types.h>

#ifndef offsetof
#define offsetof(TYPE, MEMBER) ((size_t) & ((TYPE *)0)->MEMBER)
#endif

struct key {
  __u32 src_ip;
  __u32 dst_ip;
  unsigned short src_port;
  unsigned short dst_port;
};

struct bpf_map_def SEC("maps/duration_start") duration_start = {
  .type = BPF_MAP_TYPE_HASH,
	.key_size = sizeof(struct key),
	.value_size = sizeof(__u64),
	.max_entries = 102400,

};

struct bpf_map_def SEC("maps/metrics_map") metrics_map = {
  .type = BPF_MAP_TYPE_HASH,
	.key_size = sizeof(__u32),
	.value_size = sizeof(__u64),
	.max_entries = 1024,

};

SEC("socket")
int http_filter(struct __sk_buff *skb){

  struct key key;
  struct key inv_key;

  if (load_half(skb, offsetof(struct ethhdr, h_proto)) != ETH_P_IP)
     return 0;

  if (load_byte(skb, ETH_HLEN + offsetof(struct iphdr, protocol)) != IPPROTO_TCP) {
    return 0;
  }

  struct iphdr iph;
  bpf_skb_load_bytes(skb, ETH_HLEN, &iph, sizeof(iph));

  /* Check that the packet contains the IP address for which we need the traffic */
  __u32 filter_ip_key = 1;
  __u32 *us_ipAddress;
  us_ipAddress = bpf_map_lookup_elem(&metrics_map, &filter_ip_key);


  if (us_ipAddress == NULL) {return 0;}
  if (iph.saddr != *us_ipAddress && iph.daddr != *us_ipAddress) {goto END;}


  struct tcphdr tcph;
  bpf_skb_load_bytes(skb, ETH_HLEN + sizeof(iph), &tcph, sizeof(tcph));

  __u32 tcp_hlen = tcph.doff;
  __u32 ip_hlen = iph.ihl;
  __u32 poffset = 0;
  __u32 plength = 0;
  __u32 ip_total_length = iph.tot_len;

  key.src_ip = iph.saddr;
  key.dst_ip = iph.daddr;
  key.src_port = tcph.source;
  key.dst_port = tcph.dest;

  inv_key.src_ip = key.dst_ip;
  inv_key.dst_ip = key.src_ip;
  inv_key.src_port = key.dst_port;
  inv_key.dst_port = key.src_port;

  ip_hlen = ip_hlen << 2;
  tcp_hlen = tcp_hlen << 2;


  poffset = ETH_HLEN + ip_hlen + tcp_hlen;
  plength = ip_total_length - ip_hlen - tcp_hlen;

  /* We need to check this because if offset is not greater than skb->len it
  means there is not payload and the call to load_byte will fail. We also keep
  duration_start map from filling up if there are calls to non http ports. */
  if (skb->len <= poffset){
    if ((key.src_ip == *us_ipAddress) && ((tcph.fin==1)||(tcph.rst==1))) {
      bpf_map_delete_elem(&duration_start,&inv_key);
    }
    goto END;
  }
  /* This should rarely occur */
  if (plength < 7) {
    bpf_map_delete_elem(&duration_start,&inv_key);
    goto END;
  }

    unsigned long p[12];
    int i = 0;
    for (i = 0; i < 12; i++) {

      p[i] = load_byte(skb, poffset + i);
    }

    if ((p[0] == 'H') && (p[1] == 'T') && (p[2] == 'T') && (p[3] == 'P')) {


      __u32 end_key = 2;
      __u32 count_key = 3;
      __u32 err4_key = 4;
      __u32 err5_key = 5;

      __u64 *start_time, end_time, *count_val_cur, count_val_new;
      __u64 *err4_val_cur, err4_val_new, *err5_val_cur, err5_val_new;

      start_time =  bpf_map_lookup_elem(&duration_start,&inv_key);

      if (start_time == NULL){
        goto END;
      }
      /* Send microsends to userspace */
      end_time = (bpf_ktime_get_ns() - *start_time) / 1000;
      bpf_map_update_elem(&metrics_map, &end_key, &end_time, BPF_ANY);

      /* Count requests */
      count_val_cur =  bpf_map_lookup_elem(&metrics_map,&count_key);
      if (count_val_cur == NULL) {
        count_val_new = 1;
      } else {
        count_val_new = *count_val_cur + 1 ;
      }
      bpf_map_update_elem(&metrics_map, &count_key, &count_val_new, BPF_ANY);

      /* Check the 10th byte. It should be the first digit of the status code.
      If it's not 4 or 5 we consider it's ok */
      switch (p[9]){
        case '4':

        err4_val_cur =  bpf_map_lookup_elem(&metrics_map,&err4_key);
        if (err4_val_cur == NULL) {
          err4_val_new = 1;
        } else {
          err4_val_new = *err4_val_cur + 1;
        }
        bpf_map_update_elem(&metrics_map, &err4_key, &err4_val_new, BPF_ANY);
        break;

        case '5':

        err5_val_cur =  bpf_map_lookup_elem(&metrics_map,&err5_key);
        if (err5_val_cur == NULL) {
          err5_val_new = 1;
        } else {
          err5_val_new = *err5_val_cur + 1;
        }
        bpf_map_update_elem(&metrics_map, &err5_key, &err5_val_new, BPF_ANY);
        break;

      }

      bpf_map_delete_elem(&duration_start,&inv_key);
      goto END;

    }
    //GET
  	if ((p[0] == 'G') && (p[1] == 'E') && (p[2] == 'T')) {
      if (key.src_ip == *us_ipAddress) {
        goto END;
      }
      goto HTTP_MATCH;
  	}
  	//POST
  	if ((p[0] == 'P') && (p[1] == 'O') && (p[2] == 'S') && (p[3] == 'T')) {
      if (key.src_ip == *us_ipAddress) {
        goto END;
      }
      goto HTTP_MATCH;
  	}
  	//PUT
  	if ((p[0] == 'P') && (p[1] == 'U') && (p[2] == 'T')) {
      if (key.src_ip == *us_ipAddress) {
        goto END;
      }
      goto HTTP_MATCH;
  	}
  	//DELETE
  	if ((p[0] == 'D') && (p[1] == 'E') && (p[2] == 'L') && (p[3] == 'E') && (p[4] == 'T') && (p[5] == 'E')) {
      if (key.src_ip == *us_ipAddress) {
        goto END;
      }
      goto HTTP_MATCH;
  	}
  	//HEAD
  	if ((p[0] == 'H') && (p[1] == 'E') && (p[2] == 'A') && (p[3] == 'D')) {
      if (key.src_ip == *us_ipAddress) {
        goto END;
      }
      goto HTTP_MATCH;
  	}
    //CONNECT
  	if ((p[0] == 'C') && (p[1] == 'O') && (p[2] == 'N') && (p[3] == 'N') && (p[4] == 'E') && (p[5] == 'C') && (p[6] == 'T')) {
      if (key.src_ip == *us_ipAddress) {
        goto END;
      }
      goto HTTP_MATCH;
  	}
    //OPTIONS
  	if ((p[0] == 'O') && (p[1] == 'P') && (p[2] == 'T') && (p[3] == 'I') && (p[4] == 'O') && (p[5] == 'N') && (p[6] == 'S')) {
      if (key.src_ip == *us_ipAddress) {
        goto END;
      }
      goto HTTP_MATCH;
  	}
    //TRACE
  	if ((p[0] == 'T') && (p[1] == 'R') && (p[2] == 'A') && (p[3] == 'C') && (p[4] == 'E')) {
      if (key.src_ip == *us_ipAddress) {
        goto END;
      }
      goto HTTP_MATCH;
  	}
    //PATCH
  	if ((p[0] == 'P') && (p[1] == 'A') && (p[2] == 'T') && (p[3] == 'C') && (p[4] == 'H')) {
      if (key.src_ip == *us_ipAddress) {
        goto END;
      }
      goto HTTP_MATCH;
  	}
    bpf_map_delete_elem(&duration_start,&inv_key);
    goto END;

    HTTP_MATCH:;
    __u64 ts;
    ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&duration_start, &key, &ts, BPF_ANY);
END:
return 0;
}

char _license[] SEC("license") = "GPL";

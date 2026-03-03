pub mod ztna {
    pub mod controlplane {
        pub mod v1 {
            tonic::include_proto!("ztna.controlplane.v1");
        }
    }

    pub mod dataplane {
        pub mod v1 {
            tonic::include_proto!("ztna.dataplane.v1");
        }
    }
}
